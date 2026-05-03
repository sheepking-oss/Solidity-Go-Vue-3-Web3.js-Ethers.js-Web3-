// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract SupplyChainTraceability {
    bytes32 public constant OWNER_ROLE = keccak256("OWNER_ROLE");
    bytes32 public constant MANUFACTURER_ROLE = keccak256("MANUFACTURER_ROLE");
    bytes32 public constant LOGISTICS_ROLE = keccak256("LOGISTICS_ROLE");
    bytes32 public constant RECEIVER_ROLE = keccak256("RECEIVER_ROLE");

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

    struct RoleData {
        mapping(address => bool) members;
        bytes32 adminRole;
    }

    mapping(bytes32 => RoleData) private _roles;
    mapping(bytes32 => bytes32) public latestProductHash;
    mapping(bytes32 => StateChange) public stateChanges;
    mapping(bytes32 => bytes32) public productSerialToHash;

    event ProductStateChanged(
        bytes32 indexed productHash,
        bytes32 indexed serialNumberHash,
        ProductStatus status,
        address indexed operator,
        uint256 timestamp,
        bytes32 previousHash,
        bytes32 currentHash,
        uint256 blockNumber
    );

    event RoleGranted(
        bytes32 indexed role,
        address indexed account,
        address indexed sender
    );

    event RoleRevoked(
        bytes32 indexed role,
        address indexed account,
        address indexed sender
    );

    event RoleAdminChanged(
        bytes32 indexed role,
        bytes32 indexed previousAdminRole,
        bytes32 indexed newAdminRole
    );

    modifier onlyRole(bytes32 role) {
        require(hasRole(role, msg.sender), 
            string(abi.encodePacked("AccessControl: account ", toHexString(msg.sender), " is missing role ", toHexString(role)))
        );
        _;
    }

    modifier onlyValidSerialNumber(string calldata serialNumber) {
        require(bytes(serialNumber).length > 0, "Serial number cannot be empty");
        _;
    }

    constructor() {
        _grantRole(OWNER_ROLE, msg.sender);
        _setRoleAdmin(MANUFACTURER_ROLE, OWNER_ROLE);
        _setRoleAdmin(LOGISTICS_ROLE, OWNER_ROLE);
        _setRoleAdmin(RECEIVER_ROLE, OWNER_ROLE);
        _setRoleAdmin(OWNER_ROLE, OWNER_ROLE);
        
        emit RoleGranted(OWNER_ROLE, msg.sender, msg.sender);
    }

    function hasRole(bytes32 role, address account) public view returns (bool) {
        return _roles[role].members[account];
    }

    function getRoleAdmin(bytes32 role) public view returns (bytes32) {
        return _roles[role].adminRole;
    }

    function grantRole(bytes32 role, address account) public onlyRole(getRoleAdmin(role)) {
        _grantRole(role, account);
    }

    function revokeRole(bytes32 role, address account) public onlyRole(getRoleAdmin(role)) {
        _revokeRole(role, account);
    }

    function renounceRole(bytes32 role, address account) public {
        require(account == msg.sender, "AccessControl: can only renounce roles for self");
        _revokeRole(role, account);
    }

    function _setRoleAdmin(bytes32 role, bytes32 adminRole) internal {
        bytes32 previousAdminRole = getRoleAdmin(role);
        _roles[role].adminRole = adminRole;
        emit RoleAdminChanged(role, previousAdminRole, adminRole);
    }

    function _grantRole(bytes32 role, address account) internal {
        if (!hasRole(role, account)) {
            _roles[role].members[account] = true;
            emit RoleGranted(role, account, msg.sender);
        }
    }

    function _revokeRole(bytes32 role, address account) internal {
        if (hasRole(role, account)) {
            _roles[role].members[account] = false;
            emit RoleRevoked(role, account, msg.sender);
        }
    }

    function hashSerialNumber(string calldata serialNumber) public pure returns (bytes32) {
        return keccak256(abi.encodePacked(serialNumber));
    }

    function recordManufacture(
        string calldata serialNumber,
        string calldata productInfo
    ) external onlyRole(MANUFACTURER_ROLE) onlyValidSerialNumber(serialNumber) {
        bytes32 serialNumberHash = hashSerialNumber(serialNumber);
        bytes32 previousHash = latestProductHash[serialNumberHash];
        
        require(previousHash == bytes32(0), "Product already exists");

        bytes32 currentHash = keccak256(abi.encodePacked(
            serialNumberHash,
            ProductStatus.Manufactured,
            msg.sender,
            block.timestamp,
            productInfo,
            previousHash,
            block.number
        ));

        stateChanges[currentHash] = StateChange({
            productHash: serialNumberHash,
            status: ProductStatus.Manufactured,
            operator: msg.sender,
            timestamp: block.timestamp,
            previousHash: previousHash
        });

        latestProductHash[serialNumberHash] = currentHash;
        productSerialToHash[serialNumberHash] = serialNumberHash;

        emit ProductStateChanged(
            serialNumberHash,
            serialNumberHash,
            ProductStatus.Manufactured,
            msg.sender,
            block.timestamp,
            previousHash,
            currentHash,
            block.number
        );
    }

    function recordShipment(
        string calldata serialNumber,
        string calldata shipmentInfo
    ) external onlyRole(LOGISTICS_ROLE) onlyValidSerialNumber(serialNumber) {
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
            previousHash,
            block.number
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
            currentHash,
            block.number
        );
    }

    function recordDelivery(
        string calldata serialNumber,
        string calldata deliveryInfo
    ) external onlyRole(RECEIVER_ROLE) onlyValidSerialNumber(serialNumber) {
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
            previousHash,
            block.number
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
            currentHash,
            block.number
        );
    }

    function batchGrantManufacturerRole(address[] calldata accounts) external onlyRole(OWNER_ROLE) {
        for (uint256 i = 0; i < accounts.length; i++) {
            _grantRole(MANUFACTURER_ROLE, accounts[i]);
        }
    }

    function batchGrantLogisticsRole(address[] calldata accounts) external onlyRole(OWNER_ROLE) {
        for (uint256 i = 0; i < accounts.length; i++) {
            _grantRole(LOGISTICS_ROLE, accounts[i]);
        }
    }

    function batchGrantReceiverRole(address[] calldata accounts) external onlyRole(OWNER_ROLE) {
        for (uint256 i = 0; i < accounts.length; i++) {
            _grantRole(RECEIVER_ROLE, accounts[i]);
        }
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

    function toHexString(address account) internal pure returns (string memory) {
        bytes32 value = bytes32(uint256(uint160(account)));
        bytes memory alphabet = "0123456789abcdef";
        
        bytes memory str = new bytes(42);
        str[0] = '0';
        str[1] = 'x';
        
        for (uint256 i = 0; i < 20; i++) {
            str[2 + i * 2] = alphabet[uint8(value[i + 12] >> 4)];
            str[3 + i * 2] = alphabet[uint8(value[i + 12] & 0x0f)];
        }
        
        return string(str);
    }

    function toHexString(bytes32 value) internal pure returns (string memory) {
        bytes memory alphabet = "0123456789abcdef";
        
        bytes memory str = new bytes(66);
        str[0] = '0';
        str[1] = 'x';
        
        for (uint256 i = 0; i < 32; i++) {
            str[2 + i * 2] = alphabet[uint8(value[i] >> 4)];
            str[3 + i * 2] = alphabet[uint8(value[i] & 0x0f)];
        }
        
        return string(str);
    }
}
