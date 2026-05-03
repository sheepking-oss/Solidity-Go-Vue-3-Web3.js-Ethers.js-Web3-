const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("SupplyChainTraceability", function () {
  let SupplyChainTraceability;
  let supplyChain;
  let owner;
  let addr1;
  let addr2;

  beforeEach(async function () {
    SupplyChainTraceability = await ethers.getContractFactory("SupplyChainTraceability");
    [owner, addr1, addr2] = await ethers.getSigners();
    supplyChain = await SupplyChainTraceability.deploy();
    await supplyChain.deployed();
  });

  describe("Serial Number Hashing", function () {
    it("Should correctly hash a serial number", async function () {
      const serialNumber = "PROD-2024-001";
      const hash = await supplyChain.hashSerialNumber(serialNumber);
      expect(hash).to.not.equal(ethers.constants.HashZero);
    });
  });

  describe("Product Manufacture", function () {
    it("Should record a product manufacture", async function () {
      const serialNumber = "PROD-2024-001";
      const productInfo = "High quality electronic device";

      const tx = await supplyChain.recordManufacture(serialNumber, productInfo);
      const receipt = await tx.wait();

      const event = receipt.events?.find(e => e.event === "ProductStateChanged");
      expect(event).to.exist;
      expect(event.args?.status).to.equal(0); 
    });

    it("Should reject duplicate manufacture", async function () {
      const serialNumber = "PROD-2024-001";
      const productInfo = "High quality electronic device";

      await supplyChain.recordManufacture(serialNumber, productInfo);
      
      await expect(
        supplyChain.recordManufacture(serialNumber, productInfo)
      ).to.be.revertedWith("Product already exists");
    });

    it("Should reject empty serial number", async function () {
      await expect(
        supplyChain.recordManufacture("", "Test info")
      ).to.be.revertedWith("Serial number cannot be empty");
    });
  });

  describe("Product Shipment", function () {
    beforeEach(async function () {
      const serialNumber = "PROD-2024-001";
      const productInfo = "High quality electronic device";
      await supplyChain.recordManufacture(serialNumber, productInfo);
    });

    it("Should record a product shipment", async function () {
      const serialNumber = "PROD-2024-001";
      const shipmentInfo = "Shipped via express delivery";

      const tx = await supplyChain.recordShipment(serialNumber, shipmentInfo);
      const receipt = await tx.wait();

      const event = receipt.events?.find(e => e.event === "ProductStateChanged");
      expect(event).to.exist;
      expect(event.args?.status).to.equal(1); 
    });

    it("Should reject shipment for non-existent product", async function () {
      await expect(
        supplyChain.recordShipment("NON-EXISTENT", "Test info")
      ).to.be.revertedWith("Product does not exist");
    });
  });

  describe("Product Delivery", function () {
    beforeEach(async function () {
      const serialNumber = "PROD-2024-001";
      const productInfo = "High quality electronic device";
      await supplyChain.recordManufacture(serialNumber, productInfo);
      await supplyChain.recordShipment(serialNumber, "Shipped via express");
    });

    it("Should record a product delivery", async function () {
      const serialNumber = "PROD-2024-001";
      const deliveryInfo = "Delivered to recipient";

      const tx = await supplyChain.recordDelivery(serialNumber, deliveryInfo);
      const receipt = await tx.wait();

      const event = receipt.events?.find(e => e.event === "ProductStateChanged");
      expect(event).to.exist;
      expect(event.args?.status).to.equal(2); 
    });

    it("Should reject delivery for non-existent product", async function () {
      await expect(
        supplyChain.recordDelivery("NON-EXISTENT", "Test info")
      ).to.be.revertedWith("Product does not exist");
    });
  });

  describe("State Query Functions", function () {
    it("Should get latest state hash", async function () {
      const serialNumber = "PROD-2024-001";
      const serialHash = await supplyChain.hashSerialNumber(serialNumber);
      
      await supplyChain.recordManufacture(serialNumber, "Test product");
      
      const latestHash = await supplyChain.getLatestStateHash(serialHash);
      expect(latestHash).to.not.equal(ethers.constants.HashZero);
    });

    it("Should get state change by hash", async function () {
      const serialNumber = "PROD-2024-001";
      const serialHash = await supplyChain.hashSerialNumber(serialNumber);
      
      const tx = await supplyChain.recordManufacture(serialNumber, "Test product");
      const receipt = await tx.wait();
      
      const event = receipt.events?.find(e => e.event === "ProductStateChanged");
      const currentHash = event.args?.currentHash;
      
      const stateChange = await supplyChain.getStateChange(currentHash);
      expect(stateChange.status).to.equal(0); 
      expect(stateChange.operator).to.equal(owner.address);
    });
  });

  describe("Status to String", function () {
    it("Should convert status enum to string", async function () {
      expect(await supplyChain.statusToString(0)).to.equal("Manufactured");
      expect(await supplyChain.statusToString(1)).to.equal("Shipped");
      expect(await supplyChain.statusToString(2)).to.equal("Delivered");
    });
  });
});
