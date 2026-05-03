package ethereum

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"supply-chain-indexer/config"
	"supply-chain-indexer/database"
	"supply-chain-indexer/models"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type Indexer struct {
	client          *ethclient.Client
	contractAddress common.Address
	contractABI     abi.ABI
	lastProcessedBlock uint64
}

func NewIndexer(cfg *config.Config) (*Indexer, error) {
	client, err := ethclient.Dial(cfg.EthRPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %v", err)
	}

	if cfg.ContractAddress == "" {
		return nil, fmt.Errorf("contract address not configured")
	}

	contractAddress := common.HexToAddress(cfg.ContractAddress)

	contractABI, err := getContractABI()
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %v", err)
	}

	return &Indexer{
		client:               client,
		contractAddress:      contractAddress,
		contractABI:          contractABI,
		lastProcessedBlock:   cfg.StartBlock,
	}, nil
}

func getContractABI() (abi.ABI, error) {
	abiJSON := `[
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "bytes32",
					"name": "productHash",
					"type": "bytes32"
				},
				{
					"indexed": true,
					"internalType": "bytes32",
					"name": "serialNumberHash",
					"type": "bytes32"
				},
				{
					"indexed": false,
					"internalType": "enum SupplyChainTraceability.ProductStatus",
					"name": "status",
					"type": "uint8"
				},
				{
					"indexed": false,
					"internalType": "address",
					"name": "operator",
					"type": "address"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "timestamp",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "bytes32",
					"name": "previousHash",
					"type": "bytes32"
				},
				{
					"indexed": false,
					"internalType": "bytes32",
					"name": "currentHash",
					"type": "bytes32"
				}
			],
			"name": "ProductStateChanged",
			"type": "event"
		}
	]`
	
	return abi.JSON(strings.NewReader(abiJSON))
}

func (i *Indexer) Start(ctx context.Context) error {
	log.Println("Starting event indexer...")

	eventSignature := i.contractABI.Events["ProductStateChanged"].ID

	query := ethereum.FilterQuery{
		Addresses: []common.Address{i.contractAddress},
		Topics:    [][]common.Hash{{eventSignature}},
	}

	logs := make(chan types.Log)

	sub, err := i.client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		return fmt.Errorf("failed to subscribe to logs: %v", err)
	}
	defer sub.Unsubscribe()

	log.Println("Indexer started, listening for events...")

	go i.syncPastEvents(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Indexer shutting down...")
			return nil
		case err := <-sub.Err():
			log.Printf("Subscription error: %v\n", err)
			return err
		case vLog := <-logs:
			i.processLog(vLog)
		}
	}
}

func (i *Indexer) syncPastEvents(ctx context.Context) {
	log.Println("Syncing past events...")

	currentBlock, err := i.client.BlockNumber(ctx)
	if err != nil {
		log.Printf("Failed to get current block: %v\n", err)
		return
	}

	if i.lastProcessedBlock >= currentBlock {
		log.Println("No past events to sync")
		return
	}

	eventSignature := i.contractABI.Events["ProductStateChanged"].ID

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(i.lastProcessedBlock)),
		ToBlock:   big.NewInt(int64(currentBlock)),
		Addresses: []common.Address{i.contractAddress},
		Topics:    [][]common.Hash{{eventSignature}},
	}

	logs, err := i.client.FilterLogs(ctx, query)
	if err != nil {
		log.Printf("Failed to filter past logs: %v\n", err)
		return
	}

	log.Printf("Found %d past events\n", len(logs))

	for _, vLog := range logs {
		i.processLog(vLog)
	}

	log.Println("Past events sync completed")
}

func (i *Indexer) processLog(vLog types.Log) {
	event := make(map[string]interface{})

	err := i.contractABI.UnpackIntoMap(event, "ProductStateChanged", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack log: %v\n", err)
		return
	}

	var productHash [32]byte
	var serialNumberHash [32]byte

	if len(vLog.Topics) > 1 {
		copy(productHash[:], vLog.Topics[1][:])
	}
	if len(vLog.Topics) > 2 {
		copy(serialNumberHash[:], vLog.Topics[2][:])
	}

	status, ok := event["status"].(uint8)
	if !ok {
		log.Printf("Failed to parse status\n")
		return
	}

	operator, ok := event["operator"].(common.Address)
	if !ok {
		log.Printf("Failed to parse operator\n")
		return
	}

	timestamp, ok := event["timestamp"].(*big.Int)
	if !ok {
		log.Printf("Failed to parse timestamp\n")
		return
	}

	previousHash, ok := event["previousHash"].([32]byte)
	if !ok {
		log.Printf("Failed to parse previousHash\n")
		return
	}

	currentHash, ok := event["currentHash"].([32]byte)
	if !ok {
		log.Printf("Failed to parse currentHash\n")
		return
	}

	productState := &models.ProductState{
		ProductHash:     "0x" + hex.EncodeToString(productHash[:]),
		SerialNumber:    "", 
		CurrentHash:     "0x" + hex.EncodeToString(currentHash[:]),
		PreviousHash:    "0x" + hex.EncodeToString(previousHash[:]),
		Status:          models.ProductStatus(status),
		StatusText:      models.ProductStatus(status).String(),
		Operator:        operator.Hex(),
		Timestamp:       timestamp.Int64(),
		BlockNumber:     vLog.BlockNumber,
		TransactionHash: vLog.TxHash.Hex(),
		EventIndex:      vLog.Index,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	var existing models.ProductState
	err = database.DB.Where("current_hash = ?", productState.CurrentHash).First(&existing).Error
	if err == nil {
		log.Printf("Event already processed: %s\n", productState.CurrentHash)
		return
	}

	if err != gorm.ErrRecordNotFound {
		log.Printf("Error checking existing record: %v\n", err)
		return
	}

	if err := database.DB.Create(productState).Error; err != nil {
		log.Printf("Failed to save product state: %v\n", err)
		return
	}

	log.Printf("Processed event: status=%s, productHash=%s, block=%d\n",
		productState.StatusText, productState.ProductHash, vLog.BlockNumber)
}

func Bytes32ToHex(b [32]byte) string {
	return "0x" + hex.EncodeToString(b[:])
}
