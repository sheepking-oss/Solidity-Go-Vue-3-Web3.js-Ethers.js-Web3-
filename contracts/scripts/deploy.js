async function main() {
  const [deployer] = await ethers.getSigners();

  console.log("Deploying contracts with the account:", deployer.address);
  console.log("Account balance:", (await deployer.getBalance()).toString());

  const SupplyChainTraceability = await ethers.getContractFactory("SupplyChainTraceability");
  const supplyChain = await SupplyChainTraceability.deploy();

  await supplyChain.deployed();

  console.log("SupplyChainTraceability contract deployed to:", supplyChain.address);
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
