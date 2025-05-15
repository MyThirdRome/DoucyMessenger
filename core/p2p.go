package core

// SetBroadcastTxFunc sets the function to broadcast transactions
func (bc *Blockchain) SetBroadcastTxFunc(fn func(tx interface{}) error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.broadcastTx = fn
}