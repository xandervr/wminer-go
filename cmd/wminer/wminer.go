package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"wsb.com/wminer/internals/config"
	"wsb.com/wminer/internals/helpers"
)

type Miner struct {
	ctx                 context.Context
	Version             string
	PreviousHash        string
	CurrentPreviousHash string
	Difficulty          float64
	BlockSize           float64
	BlockReward         float64
	MinerAddress        string
	NodeHost            string
	NodePort            int64
	Threads             int
}

func NewMiner(ctx context.Context, host string, port int64, address string, threads int) *Miner {
	m := new(Miner)
	m.ctx = ctx
	m.MinerAddress = address
	m.NodeHost = host
	m.NodePort = port
	m.Threads = threads
	return m
}

func (m *Miner) generateBaseHttp() string {
	return "http://" + m.NodeHost + ":" + strconv.FormatInt(m.NodePort, 10)
}

func (m *Miner) GetChainInfo() {
	resp, err := http.Get(m.generateBaseHttp() + "/info")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var data map[string]interface{}
	json.Unmarshal([]byte(body), &data)

	if data != nil {
		m.Version = data["version"].(string)
		m.PreviousHash = data["previous_hash"].(string)
		m.Difficulty = data["difficulty"].(float64)
		m.BlockSize = data["block_size"].(float64)
		m.BlockReward = data["block_reward"].(float64)
	}
}

func (m *Miner) PollChainInfo() {
	ticker := time.NewTicker(time.Duration(15) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		select {
		default:
			m.GetChainInfo()
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *Miner) GetTransactions() []*helpers.Transaction {
	resp, err := http.Get(m.generateBaseHttp() + "/transactions")
	if err != nil {
		fmt.Println(err)
		return make([]*helpers.Transaction, 0)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var data []*helpers.Transaction
	json.Unmarshal([]byte(body), &data)

	return data
}

func (m *Miner) sendBlock(block *helpers.Block) *helpers.Block {
	jsonBlock, err := json.Marshal(block)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	req, err := http.NewRequest("POST", m.generateBaseHttp()+"/blocks", bytes.NewBuffer(jsonBlock))
	if err != nil {
		fmt.Println(err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var data *helpers.Block
	json.Unmarshal([]byte(body), &data)
	return data
}

func (m *Miner) generateBaseString(merkleRoot string, timestamp int64) string {
	return m.Version + helpers.LittleEndian(m.PreviousHash) + helpers.LittleEndian(merkleRoot) + helpers.LittleEndian(helpers.HexInt(timestamp)) + helpers.LittleEndian(helpers.HexFloat64(m.Difficulty))
}

func (m *Miner) assembleBlock(transactions []*helpers.Transaction, timestamp int64) (string, []*helpers.Transaction) {
	idx := 0
	totalFee := 0.0
	chosenTransactions := make([]*helpers.Transaction, 0)
	coinbaseTransaction := new(helpers.Transaction)
	coinbaseTransaction.Timestamp = time.Now().Unix()
	coinbaseTransaction.Sender = "coinbase"
	coinbaseTransaction.Receiver = m.MinerAddress
	coinbaseTransaction.Amount = 0
	coinbaseTransaction.Fee = 0
	coinbaseTransaction.Message = ""
	coinbaseTransaction.Signature = ""
	coinbaseTransaction.Pubkey = ""
	minsize := float64(unsafe.Sizeof(coinbaseTransaction)) - float64(unsafe.Sizeof(1.0))
	m.BlockSize -= minsize
	for idx < len(transactions) && m.BlockSize >= minsize {
		tx := transactions[idx]
		totalFee += tx.Fee
		m.BlockSize -= float64(unsafe.Sizeof(tx))
		chosenTransactions = append(chosenTransactions, tx)
		idx++
	}
	coinbaseTransaction.Amount = m.BlockReward + totalFee
	chosenTransactions = append([]*helpers.Transaction{coinbaseTransaction}, chosenTransactions...)
	merkleRoot := helpers.GenerateMerkleRoot(chosenTransactions)
	baseString := m.generateBaseString(merkleRoot, timestamp)
	return baseString, chosenTransactions
}

func (m *Miner) calculateHashrate(hashes int, time int64, nonce int) {
	if time == 0 {
		time = 1
	}
	var round func(n float64) float64
	round = func(n float64) float64 {
		return math.Floor(n*100) / 100
	}

	hashrate := float64(hashes) / float64(time) * float64(m.Threads)
	formatted := ""
	if hashrate >= 1000 && hashrate < 1000*1000 {
		formatted = fmt.Sprintf("%.2f Kh/s", round(hashrate/1000))
	} else if hashrate < 1000 {
		formatted = fmt.Sprintf("%.2f h/s", round(hashrate))
	} else if hashrate >= 1000*1000 && hashrate < 1000*1000*1000 {
		formatted = fmt.Sprintf("%.2f Mh/s", round(hashrate/1000/1000))
	}

	fmt.Printf("%s N: %d\r", formatted, nonce)
}

func (m *Miner) isDifficultEnough(hash string) bool {
	hashInt := new(big.Int)
	hashInt.SetString(hash, 16)
	diffInt := new(big.Int)
	diffInt.SetString(fmt.Sprintf("%f", helpers.CalculateDifficulty(m.Difficulty)), 10)
	return hashInt.Cmp(diffInt) == -1
}

func (m *Miner) formatPrintBlock(b *helpers.Block) {
	fmt.Printf("\nBLOCK %d\n", b.Height)
	fmt.Printf("Hash: %s\n", b.Hash)
	fmt.Printf("Previous hash: %s\n", b.PreviousHash)
	fmt.Printf("Merkle root: %s\n", b.MerkleRoot)
	fmt.Printf("Difficulty: %f\n", b.Difficulty)
	fmt.Printf("Nonce: %d\n", b.Nonce)
	fmt.Printf("Timestamp: %d\n", b.Timestamp)
	fmt.Printf("\n")
}

func (m *Miner) mineBlock(template string, transactions []*helpers.Transaction, timestamp int64) int64 {
	m.CurrentPreviousHash = m.PreviousHash
	fmt.Printf("Mining block with previous hash %s TS: %d\n", m.CurrentPreviousHash, timestamp)
	nonce := 1
	hashCount := 0
	start := time.Now().Unix()
	for m.CurrentPreviousHash == m.PreviousHash {
		hash := helpers.LittleEndian(helpers.SerializeSHA256(helpers.SerializeSHA256(fmt.Sprintf("%s%s", template, helpers.LittleEndian(helpers.HexInt(int64(nonce)))))))
		hashCount++
		intermediateTime := time.Now().Unix() - start
		m.calculateHashrate(hashCount, intermediateTime, nonce)

		if m.isDifficultEnough(hash) {
			end := time.Now().Unix() - start
			fmt.Printf("FOUND HASH: %s NONCE: %d in %d seconds\n", hash, nonce, end)
			block := new(helpers.Block)
			block.Transactions = transactions
			block.Nonce = nonce
			block.Timestamp = timestamp
			b := m.sendBlock(block)
			m.formatPrintBlock(b)
			return time.Now().Unix()
		} else {
			nonce++
		}
	}
	return time.Now().Unix()
}

func (m *Miner) StartMiner(initialTimestamp int64) {
	go m.PollChainInfo()
	timestamp := initialTimestamp
	if initialTimestamp == 0 {
		timestamp = time.Now().Unix()
	}
	for {
		select {
		default:
			m.GetChainInfo()
			txs := m.GetTransactions()

			template, chosenTxs := m.assembleBlock(txs, timestamp)
			timestamp = m.mineBlock(template, chosenTxs, timestamp)
		case <-m.ctx.Done():
			return
		}
	}
}

func showBanner(address string) {
	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	fmt.Println("@ ===                               WMiner v2.1                   			 === @")
	fmt.Printf("@ === WALLET ADDRESS: %s === @\n", address)
	fmt.Println("@ ===                                                                                    === @")
	fmt.Println("@ ===                               AUTHOR: Koen Brekat                                  === @")
	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
}

func main() {
	config, err := config.LoadConfiguration("./config.json")
	if err != nil {
		panic(err)
	}
	showBanner(config.WalletAddress)
	ctx := context.Background()
	m := NewMiner(ctx, config.NodeHost, config.NodePort, config.WalletAddress, config.Threads)

	m.GetChainInfo()
	initial := time.Now().Unix()
	for i := 0; i < config.Threads; i++ {
		go m.StartMiner(initial + int64(i))
	}
	// go m.StartMiner(0)
	termChan := make(chan os.Signal)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)
	<-termChan
}
