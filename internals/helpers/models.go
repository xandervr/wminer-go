package helpers

type Transaction struct {
	Timestamp int64   `json:"timestamp"`
	Sender    string  `json:"sender"`
	Receiver  string  `json:"receiver"`
	Amount    float64 `json:"amount"`
	Fee       float64 `json:"fee"`
	Message   string  `json:"message"`
	Signature string  `json:"signature"`
	Pubkey    string  `json:"pubkey"`
}

type Block struct {
	Height       int64          `json:"height"`
	Version      string         `json:"version"`
	PreviousHash string         `json:"previous_hash"`
	MerkleRoot   string         `json:"merkle_root"`
	Difficulty   float64        `json:"difficulty"`
	Hash         string         `json:"hash"`
	Timestamp    int64          `json:"timestamp"`
	Nonce        int            `json:"nonce"`
	Transactions []*Transaction `json:"transactions"`
}