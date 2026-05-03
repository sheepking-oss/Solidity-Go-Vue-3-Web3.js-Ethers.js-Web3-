// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract SupplyChainTraceability {
    enum ProductStatus {
        Manufactured,
        Shipped,
        Delivered
    }

    struct StateChange {
        bytes32 productHash;
        ProductStatus status;
        address operator;
        uint256 timestamp;
        bytes32 previousHash;
    }

    mapping(bytes32 => bytes32) public latestProductHash;
    mapping(bytes32 => StateChange) public stateChanges;

    event ProductStateChanged(
        bytes32 indexed productHash,
        bytes32 indexed serialNumberHash,
        ProductStatus status,
        address operator,
        uint256 timestamp,
        bytes32 previousHash,
        bytes32 currentHash
    );

    modifier onlyValidSerialNumber(string calldata serialNumber) {
        require(bytes(serialNumber).length > 0, "Serial number cannot be empty");
        _;
    }

    function hashSerialNumber(string calldata serialNumber) public pure returns (bytes32) {
        return keccak256(abi.encodePacked(serialNumber));
    }

    function recordManufacture(
        string calldata serialNumber,
        string calldata productInfo
    ) external onlyValidSerialNumber(serialNumber) {
        bytes32 serialNumberHash = hashSerialNumber(serialNumber);
        bytes32 previousHash = latestProductHash[serialNumberHash];
        
        require(previousHash == bytes32(0), "Product already exists");

        bytes32 currentHash = keccak256(abi.encodePacked(
            serialNumberHash,
            ProductStatus.Manufactured,
            msg.sender,
            block.timestamp,
            productInfo,
            previousHash
        ));

        stateChanges[currentHash] = StateChange({
            productHash: serialNumberHash,
            status: ProductStatus.Manufactured,
            operator: msg.sender,
            timestamp: block.timestamp,
            previousHash: previousHash
        });

        latestProductHash[serialNumberHash] = currentHash;

        emit ProductStateChanged(
            serialNumberHash,
            serialNumberHash,
            ProductStatus.Manufactured,
            msg.sender,
            block.timestamp,
            previousHash,
            currentHash
        );
    }

    function recordShipment(
        string calldata serialNumber,
        string calldata shipmentInfo
    ) external onlyValidSerialNumber(serialNumber) {
        bytes32 serialNumberHash = hashSerialNumber(serialNumber);
        bytes32 previousHash = latestProductHash[serialNumberHash];
        
        require(previousHash != bytes32(0), "Product does not exist");
        
        StateChange storage previousState = stateChanges[previousHash];
        require(
            previousState.status == ProductStatus.Manufactured,
            "Product must be manufactured before shipping"
        );

        bytes32 currentHash = keccak256(abi.encodePacked(
            serialNumberHash,
            ProductStatus.Shipped,
            msg.sender,
            block.timestamp,
            shipmentInfo,
            previousHash
        ));

        stateChanges[currentHash] = StateChange({
            productHash: serialNumberHash,
            status: ProductStatus.Shipped,
            operator: msg.sender,
            timestamp: block.timestamp,
            previousHash: previousHash
        });

        latestProductHash[serialNumberHash] = currentHash;

        emit ProductStateChanged(
            serialNumberHash,
            serialNumberHash,
            ProductStatus.Shipped,
            msg.sender,
            block.timestamp,
            previousHash,
            currentHash
        );
    }

    function recordDelivery(
        string calldata serialNumber,
        string calldata deliveryInfo
    ) external onlyValidSerialNumber(serialNumber) {
        bytes32 serialNumberHash = hashSerialNumber(serialNumber);
        bytes32 previousHash = latestProductHash[serialNumberHash];
        
        require(previousHash != bytes32(0), "Product does not exist");
        
        StateChange storage previousState = stateChanges[previousHash];
        require(
            previousState.status == ProductStatus.Shipped,
            "Product must be shipped before delivering"
        );

        bytes32 currentHash = keccak256(abi.encodePacked(
            serialNumberHash,
            ProductStatus.Delivered,
            msg.sender,
            block.timestamp,
            deliveryInfo,
            previousHash
        ));

        stateChanges[currentHash] = StateChange({
            productHash: serialNumberHash,
            status: ProductStatus.Delivered,
            operator: msg.sender,
            timestamp: block.timestamp,
            previousHash: previousHash
        });

        latestProductHash[serialNumberHash] = currentHash;

        emit ProductStateChanged(
            serialNumberHash,
            serialNumberHash,
            ProductStatus.Delivered,
            msg.sender,
            block.timestamp,
            previousHash,
            currentHash
        );
    }

    function getLatestStateHash(bytes32 serialNumberHash) external view returns (bytes32) {
        return latestProductHash[serialNumberHash];
    }

    function getStateChange(bytes32 stateHash) external view returns (
        bytes32 productHash,
        ProductStatus status,
        address operator,
        uint256 timestamp,
        bytes32 previousHash
    ) {
        StateChange storage change = stateChanges[stateHash];
        return (
            change.productHash,
            change.status,
            change.operator,
            change.timestamp,
            change.previousHash
        );
    }

    function statusToString(ProductStatus status) external pure returns (string memory) {
        if (status == ProductStatus.Manufactured) return "Manufactured";
        if (status == ProductStatus.Shipped) return "Shipped";
        if (status == ProductStatus.Delivered) return "Delivered";
        return "Unknown";
    }
}
