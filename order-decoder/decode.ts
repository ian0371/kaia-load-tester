#!/usr/bin/env node

import { ethers } from "ethers";

// configure your RPC endpoint
const RPC_URL = process.env.RPC_URL || "http://localhost:8551";

const provider = new ethers.JsonRpcProvider(RPC_URL);

const TOPIC_TRADES = ethers.keccak256(ethers.toUtf8Bytes("Trades"));
const TOPIC_CANCEL = ethers.keccak256(ethers.toUtf8Bytes("Cancel")); 
const TOPIC_CANCELED_IDS = ethers.keccak256(ethers.toUtf8Bytes("CanceledIds"));
const TOPIC_TRIGGED_IDS = ethers.keccak256(ethers.toUtf8Bytes("TriggeredIds"));
const TOPIC_TRIGGER_ABOVE = ethers.keccak256(ethers.toUtf8Bytes("TopicTriggerAbove"));

const topicHashToName: { [key: string]: string } = {
    [TOPIC_TRADES]: "TRADES",
    [TOPIC_CANCEL]: "CANCEL",
    [TOPIC_CANCELED_IDS]: "CANCELED IDS",
    [TOPIC_TRIGGED_IDS]: "TRIGGED IDS",
    [TOPIC_TRIGGER_ABOVE]: "TRIGGED ABOVE",
};

const txTypeToName = {
    0x01: "SESSION",
    0x02: "TRANSFER",
    0x11: "TOKEN_TRANSFER",
    0x21: "NEW",
    0x22: "CANCEL",
    0x23: "CANCEL_ALL",
    0x24: "MODIFY",
    0x25: "STOP_ORDER",
    0xff: "INVALID",
};

async function parseTradeLog(data: string) {
    // decode RLP
    const decoded = ethers.decodeRlp(data);
    if (decoded.length !== 14) {
        throw new Error("Trade log data length is not 14");
    }

    // strip 0x prefix
    const buyTxHash = Buffer.from((decoded[1] as string).slice(2), "hex").toString();
    const sellTxHash = Buffer.from((decoded[2] as string).slice(2), "hex").toString();
    const priceHex = decoded[7] as string;
    const priceEth = ethers.formatEther(priceHex);
    return { buyTxHash, sellTxHash, priceHex, priceEth };
}

async function parseOrderTx(txHash: string) {
    const tx = await provider.getTransaction(txHash);
    if (!tx) {
        throw new Error("Transaction not found");
    }

    const data = Buffer.from((tx.data as string).slice(2), "hex");
    const txType = data[0];
    const ctx = JSON.parse(Buffer.from(data.subarray(1)).toString());
    return { txType, ctx };
}

async function printOrder(txType: number, ctx: any, txHash?: string) {
    let tag;
    if (txType === 0x25) {
        tag = "STOP_ORDER";
    } else if (txType == 0x21) {
        if (ctx.tpslLimit != null) {
            tag = "LIMIT_TPSL";
        } else {
            tag = "LIMIT";
        }
    }
    console.log(`....$${ctx.price} ${["BUY", "SELL"][ctx.side]} ${tag} ${txHash ? `(${txHash})` : ''}`);
}

async function parseTriggeredIdsLog(data: string) {
    const decoded = ethers.decodeRlp(data);
    const txs: string[] = [];
    for (const item of decoded) {
        txs.push(Buffer.from((item as string).slice(2), "hex").toString());
    }
    return txs;
}

async function decodeTradeTxLog(log: ethers.Log) {
    if (log.topics.length > 1) {
        throw new Error("Multiple topics found in a log");
    }

    const data = log.data;
    if (!data || data === "0x") {
        console.log("Skipping empty data field.");
        return;
    }

    const topic = log.topics[0];
    if (topic == TOPIC_TRIGGED_IDS) {
        const txs = await parseTriggeredIdsLog(data);
        for (const [i, tx] of txs.entries()) {
            console.log(`....TRIGERRED IDS[${i}]: ${tx}`);
        }
    } else if (topic == TOPIC_TRADES) {
        const { buyTxHash, sellTxHash, priceEth } = await parseTradeLog(data);
        {
            const { txType, ctx } = await parseOrderTx(buyTxHash);
            // here we just print raw hex input slices; parsing ABI requires the contract ABI
            printOrder(txType, ctx, buyTxHash);
        }
        {
            const { txType, ctx } = await parseOrderTx(sellTxHash);
            // here we just print raw hex input slices; parsing ABI requires the contract ABI
            printOrder(txType, ctx, sellTxHash);
        }
        console.log(`....matched price: $${priceEth}`);
    } else if (topic == TOPIC_TRIGGER_ABOVE) {
    } else {
        console.error(`....unexpected topic found: ${topic}, txhash: ${log.transactionHash}`);
    }
}

async function decodeTradeTxLogs(logs: readonly ethers.Log[]) {
    if (logs.length === 0) {
        console.log("..No logs found");
        return;
    }

    for (const [i, log] of logs.entries()) {
        const topic = log.topics[0];
        console.log(`..log[${i}] (topic: ${topicHashToName[topic] || 'UNKNOWN'})`);
        await decodeTradeTxLog(log);
    }
}

async function main() {
    if (process.argv.length < 3) {
        console.error(
            "Usage: ./decode.js <txhash> OR ./decode.js <fromBlock> <toBlock> <topic>",
        );
        process.exit(1);
    }

    if (process.argv.length === 3) {
        const arg = process.argv[2];

        if (arg.length !== 66 || !arg.startsWith("0x")) {
            console.error("Invalid txhash:", arg);
            process.exit(1);
        }

        const txhash = arg;
        const receipt = await provider.getTransactionReceipt(txhash);
        if (!receipt) {
            console.error("No receipt found for tx:", txhash);
            process.exit(1);
        }

        await decodeTradeTxLogs(receipt.logs);
    } else {
        // fetch logs
        const fromBlock = parseInt(process.argv[2]);
        const toBlock = parseInt(process.argv[3]);
        console.log(
            `Analyzing logs from block ${fromBlock} to block ${toBlock}`,
        );

        if (!fromBlock || !toBlock || fromBlock > toBlock) {
            console.error("Invalid block range");
            process.exit(1);
        }


        for (let blockNumber = fromBlock; blockNumber <= toBlock; blockNumber++) {
            const block = await provider.getBlock(blockNumber, true);
            if (!block) {
                console.error("No block found for block:", blockNumber);
                continue;
            }

            for (const [i, tx] of block.transactions.entries()) {
                if (i > 0) {
                    const receipt = await provider.getTransactionReceipt(tx);
                    if (!receipt) {
                        console.error("No receipt found for tx:", tx);
                        continue;
                    }
                    console.log(`Analyzing block ${blockNumber} tx[${i}]: ${tx}`);
                    await decodeTradeTxLogs(receipt.logs);
                    console.log("--------------------------------------");
                }
            }
        }
    }
}

main().catch(console.error);
