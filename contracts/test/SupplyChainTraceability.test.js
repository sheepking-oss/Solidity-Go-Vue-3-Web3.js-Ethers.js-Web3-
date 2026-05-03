const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("SupplyChainTraceability", function () {
  let SupplyChainTraceability;
  let supplyChain;
  let owner;
  let manufacturer;
  let logistics;
  let receiver;
  let unauthorized;

  const OWNER_ROLE = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("OWNER_ROLE"));
  const MANUFACTURER_ROLE = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("MANUFACTURER_ROLE"));
  const LOGISTICS_ROLE = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("LOGISTICS_ROLE"));
  const RECEIVER_ROLE = ethers.utils.keccak256(ethers.utils.toUtf8Bytes("RECEIVER_ROLE"));

  beforeEach(async function () {
    SupplyChainTraceability = await ethers.getContractFactory("SupplyChainTraceability");
    [owner, manufacturer, logistics, receiver, unauthorized] = await ethers.getSigners();
    supplyChain = await SupplyChainTraceability.deploy();
    await supplyChain.deployed();
  });

  describe("Role-Based Access Control", function () {
    describe("Deployment", function () {
      it("Should set the deployer as OWNER_ROLE", async function () {
        expect(await supplyChain.hasRole(OWNER_ROLE, owner.address)).to.be.true;
      });

      it("Should set OWNER_ROLE as admin for all roles", async function () {
        expect(await supplyChain.getRoleAdmin(MANUFACTURER_ROLE)).to.equal(OWNER_ROLE);
        expect(await supplyChain.getRoleAdmin(LOGISTICS_ROLE)).to.equal(OWNER_ROLE);
        expect(await supplyChain.getRoleAdmin(RECEIVER_ROLE)).to.equal(OWNER_ROLE);
        expect(await supplyChain.getRoleAdmin(OWNER_ROLE)).to.equal(OWNER_ROLE);
      });

      it("Should emit RoleGranted event for owner on deployment", async function () {
        const receipt = await supplyChain.deployTransaction.wait();
        const event = receipt.events?.find(e => e.event === "RoleGranted");
        
        expect(event).to.exist;
        expect(event.args?.role).to.equal(OWNER_ROLE);
        expect(event.args?.account).to.equal(owner.address);
      });
    });

    describe("Role Management", function () {
      it("Should allow owner to grant MANUFACTURER_ROLE", async function () {
        await expect(
          supplyChain.grantRole(MANUFACTURER_ROLE, manufacturer.address)
        ).to.emit(supplyChain, "RoleGranted")
         .withArgs(MANUFACTURER_ROLE, manufacturer.address, owner.address);

        expect(await supplyChain.hasRole(MANUFACTURER_ROLE, manufacturer.address)).to.be.true;
      });

      it("Should allow owner to grant LOGISTICS_ROLE", async function () {
        await expect(
          supplyChain.grantRole(LOGISTICS_ROLE, logistics.address)
        ).to.emit(supplyChain, "RoleGranted")
         .withArgs(LOGISTICS_ROLE, logistics.address, owner.address);

        expect(await supplyChain.hasRole(LOGISTICS_ROLE, logistics.address)).to.be.true;
      });

      it("Should allow owner to grant RECEIVER_ROLE", async function () {
        await expect(
          supplyChain.grantRole(RECEIVER_ROLE, receiver.address)
        ).to.emit(supplyChain, "RoleGranted")
         .withArgs(RECEIVER_ROLE, receiver.address, owner.address);

        expect(await supplyChain.hasRole(RECEIVER_ROLE, receiver.address)).to.be.true;
      });

      it("Should allow owner to revoke roles", async function () {
        await supplyChain.grantRole(MANUFACTURER_ROLE, manufacturer.address);
        
        await expect(
          supplyChain.revokeRole(MANUFACTURER_ROLE, manufacturer.address)
        ).to.emit(supplyChain, "RoleRevoked")
         .withArgs(MANUFACTURER_ROLE, manufacturer.address, owner.address);

        expect(await supplyChain.hasRole(MANUFACTURER_ROLE, manufacturer.address)).to.be.false;
      });

      it("Should allow users to renounce their own roles", async function () {
        await supplyChain.grantRole(MANUFACTURER_ROLE, manufacturer.address);
        
        await expect(
          supplyChain.connect(manufacturer).renounceRole(MANUFACTURER_ROLE, manufacturer.address)
        ).to.emit(supplyChain, "RoleRevoked")
         .withArgs(MANUFACTURER_ROLE, manufacturer.address, manufacturer.address);

        expect(await supplyChain.hasRole(MANUFACTURER_ROLE, manufacturer.address)).to.be.false;
      });

      it("Should reject unauthorized role granting", async function () {
        await expect(
          supplyChain.connect(unauthorized).grantRole(MANUFACTURER_ROLE, manufacturer.address)
        ).to.be.revertedWith("AccessControl");
      });

      it("Should reject unauthorized role revoking", async function () {
        await supplyChain.grantRole(MANUFACTURER_ROLE, manufacturer.address);
        
        await expect(
          supplyChain.connect(unauthorized).revokeRole(MANUFACTURER_ROLE, manufacturer.address)
        ).to.be.revertedWith("AccessControl");
      });

      it("Should allow batch granting of MANUFACTURER_ROLE", async function () {
        const accounts = [manufacturer.address, unauthorized.address];
        
        await expect(
          supplyChain.batchGrantManufacturerRole(accounts)
        ).to.emit(supplyChain, "RoleGranted");

        expect(await supplyChain.hasRole(MANUFACTURER_ROLE, manufacturer.address)).to.be.true;
        expect(await supplyChain.hasRole(MANUFACTURER_ROLE, unauthorized.address)).to.be.true;
      });

      it("Should allow batch granting of LOGISTICS_ROLE", async function () {
        const accounts = [logistics.address, unauthorized.address];
        
        await expect(
          supplyChain.batchGrantLogisticsRole(accounts)
        ).to.emit(supplyChain, "RoleGranted");

        expect(await supplyChain.hasRole(LOGISTICS_ROLE, logistics.address)).to.be.true;
        expect(await supplyChain.hasRole(LOGISTICS_ROLE, unauthorized.address)).to.be.true;
      });

      it("Should allow batch granting of RECEIVER_ROLE", async function () {
        const accounts = [receiver.address, unauthorized.address];
        
        await expect(
          supplyChain.batchGrantReceiverRole(accounts)
        ).to.emit(supplyChain, "RoleGranted");

        expect(await supplyChain.hasRole(RECEIVER_ROLE, receiver.address)).to.be.true;
        expect(await supplyChain.hasRole(RECEIVER_ROLE, unauthorized.address)).to.be.true;
      });

      it("Should reject batch role granting from non-owner", async function () {
        const accounts = [manufacturer.address];
        
        await expect(
          supplyChain.connect(unauthorized).batchGrantManufacturerRole(accounts)
        ).to.be.revertedWith("AccessControl");
      });
    });
  });

  describe("Product State Operations with Access Control", function () {
    beforeEach(async function () {
      await supplyChain.grantRole(MANUFACTURER_ROLE, manufacturer.address);
      await supplyChain.grantRole(LOGISTICS_ROLE, logistics.address);
      await supplyChain.grantRole(RECEIVER_ROLE, receiver.address);
    });

    describe("Product Manufacture", function () {
      it("Should allow manufacturer to record manufacture", async function () {
        const serialNumber = "PROD-2024-001";
        const productInfo = "High quality electronic device";

        const tx = await supplyChain.connect(manufacturer).recordManufacture(serialNumber, productInfo);
        const receipt = await tx.wait();

        const event = receipt.events?.find(e => e.event === "ProductStateChanged");
        expect(event).to.exist;
        expect(event.args?.status).to.equal(0);
        expect(event.args?.operator).to.equal(manufacturer.address);
      });

      it("Should reject manufacture from unauthorized account", async function () {
        const serialNumber = "PROD-2024-001";
        const productInfo = "Test product";

        await expect(
          supplyChain.connect(unauthorized).recordManufacture(serialNumber, productInfo)
        ).to.be.revertedWith("AccessControl");
      });

      it("Should reject manufacture from logistics", async function () {
        const serialNumber = "PROD-2024-001";
        const productInfo = "Test product";

        await expect(
          supplyChain.connect(logistics).recordManufacture(serialNumber, productInfo)
        ).to.be.revertedWith("AccessControl");
      });

      it("Should reject manufacture from receiver", async function () {
        const serialNumber = "PROD-2024-001";
        const productInfo = "Test product";

        await expect(
          supplyChain.connect(receiver).recordManufacture(serialNumber, productInfo)
        ).to.be.revertedWith("AccessControl");
      });
    });

    describe("Product Shipment", function () {
      beforeEach(async function () {
        await supplyChain.connect(manufacturer).recordManufacture("PROD-2024-001", "Test product");
      });

      it("Should allow logistics to record shipment", async function () {
        const serialNumber = "PROD-2024-001";
        const shipmentInfo = "Shipped via express delivery";

        const tx = await supplyChain.connect(logistics).recordShipment(serialNumber, shipmentInfo);
        const receipt = await tx.wait();

        const event = receipt.events?.find(e => e.event === "ProductStateChanged");
        expect(event).to.exist;
        expect(event.args?.status).to.equal(1);
        expect(event.args?.operator).to.equal(logistics.address);
      });

      it("Should reject shipment from unauthorized account", async function () {
        const serialNumber = "PROD-2024-001";
        const shipmentInfo = "Test shipment";

        await expect(
          supplyChain.connect(unauthorized).recordShipment(serialNumber, shipmentInfo)
        ).to.be.revertedWith("AccessControl");
      });

      it("Should reject shipment from manufacturer", async function () {
        const serialNumber = "PROD-2024-001";
        const shipmentInfo = "Test shipment";

        await expect(
          supplyChain.connect(manufacturer).recordShipment(serialNumber, shipmentInfo)
        ).to.be.revertedWith("AccessControl");
      });

      it("Should reject shipment from receiver", async function () {
        const serialNumber = "PROD-2024-001";
        const shipmentInfo = "Test shipment";

        await expect(
          supplyChain.connect(receiver).recordShipment(serialNumber, shipmentInfo)
        ).to.be.revertedWith("AccessControl");
      });
    });

    describe("Product Delivery", function () {
      beforeEach(async function () {
        await supplyChain.connect(manufacturer).recordManufacture("PROD-2024-001", "Test product");
        await supplyChain.connect(logistics).recordShipment("PROD-2024-001", "Test shipment");
      });

      it("Should allow receiver to record delivery", async function () {
        const serialNumber = "PROD-2024-001";
        const deliveryInfo = "Delivered to recipient";

        const tx = await supplyChain.connect(receiver).recordDelivery(serialNumber, deliveryInfo);
        const receipt = await tx.wait();

        const event = receipt.events?.find(e => e.event === "ProductStateChanged");
        expect(event).to.exist;
        expect(event.args?.status).to.equal(2);
        expect(event.args?.operator).to.equal(receiver.address);
      });

      it("Should reject delivery from unauthorized account", async function () {
        const serialNumber = "PROD-2024-001";
        const deliveryInfo = "Test delivery";

        await expect(
          supplyChain.connect(unauthorized).recordDelivery(serialNumber, deliveryInfo)
        ).to.be.revertedWith("AccessControl");
      });

      it("Should reject delivery from manufacturer", async function () {
        const serialNumber = "PROD-2024-001";
        const deliveryInfo = "Test delivery";

        await expect(
          supplyChain.connect(manufacturer).recordDelivery(serialNumber, deliveryInfo)
        ).to.be.revertedWith("AccessControl");
      });

      it("Should reject delivery from logistics", async function () {
        const serialNumber = "PROD-2024-001";
        const deliveryInfo = "Test delivery";

        await expect(
          supplyChain.connect(logistics).recordDelivery(serialNumber, deliveryInfo)
        ).to.be.revertedWith("AccessControl");
      });
    });

    describe("Event Data Integrity", function () {
      it("Should emit event with correct block number", async function () {
        const serialNumber = "PROD-2024-001";
        const productInfo = "Test product";

        const tx = await supplyChain.connect(manufacturer).recordManufacture(serialNumber, productInfo);
        const receipt = await tx.wait();

        const event = receipt.events?.find(e => e.event === "ProductStateChanged");
        expect(event).to.exist;
        expect(event.args?.blockNumber).to.equal(receipt.blockNumber);
      });

      it("Should include operator as indexed parameter", async function () {
        const serialNumber = "PROD-2024-001";
        const productInfo = "Test product";

        const tx = await supplyChain.connect(manufacturer).recordManufacture(serialNumber, productInfo);
        const receipt = await tx.wait();

        const event = receipt.events?.find(e => e.event === "ProductStateChanged");
        expect(event).to.exist;
        
        expect(event.topics.length).to.equal(4);
        const operatorTopic = event.topics[3];
        expect(operatorTopic.toLowerCase()).to.equal(
          ethers.utils.hexZeroPad(manufacturer.address.toLowerCase(), 32).toLowerCase()
        );
      });
    });
  });

  describe("Complete Flow Test", function () {
    it("Should complete full product lifecycle with proper roles", async function () {
      await supplyChain.grantRole(MANUFACTURER_ROLE, manufacturer.address);
      await supplyChain.grantRole(LOGISTICS_ROLE, logistics.address);
      await supplyChain.grantRole(RECEIVER_ROLE, receiver.address);

      const serialNumber = "PROD-2024-FULL-001";

      let tx = await supplyChain.connect(manufacturer).recordManufacture(serialNumber, "Premium product");
      let receipt = await tx.wait();
      let event = receipt.events?.find(e => e.event === "ProductStateChanged");
      expect(event.args?.status).to.equal(0);

      tx = await supplyChain.connect(logistics).recordShipment(serialNumber, "Express shipping");
      receipt = await tx.wait();
      event = receipt.events?.find(e => e.event === "ProductStateChanged");
      expect(event.args?.status).to.equal(1);

      tx = await supplyChain.connect(receiver).recordDelivery(serialNumber, "Signed delivery");
      receipt = await tx.wait();
      event = receipt.events?.find(e => e.event === "ProductStateChanged");
      expect(event.args?.status).to.equal(2);
    });
  });

  describe("Utility Functions", function () {
    it("Should correctly hash serial number", async function () {
      const serialNumber = "PROD-2024-001";
      const expectedHash = ethers.utils.keccak256(ethers.utils.toUtf8Bytes(serialNumber));
      
      const result = await supplyChain.hashSerialNumber(serialNumber);
      expect(result).to.equal(expectedHash);
    });

    it("Should convert status to string", async function () {
      expect(await supplyChain.statusToString(0)).to.equal("Manufactured");
      expect(await supplyChain.statusToString(1)).to.equal("Shipped");
      expect(await supplyChain.statusToString(2)).to.equal("Delivered");
      expect(await supplyChain.statusToString(3)).to.equal("Unknown");
    });
  });
});
