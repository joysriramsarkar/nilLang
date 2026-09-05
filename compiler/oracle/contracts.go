package oracle

// Contract represents formal preconditions, postconditions, or invariants
type Contract struct {
	TargetSymbol string `json:"target_symbol"`
	Requires     string `json:"requires,omitempty"`  // Precondition
	Ensures      string `json:"ensures,omitempty"`   // Postcondition
	Invariant    string `json:"invariant,omitempty"` // Invariant
	Description  string `json:"description"`
}

// AddContract adds a formal contract to the oracle
func (o *CompilerOracle) AddContract(c Contract) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.contracts = append(o.contracts, c)
}

// GetContractsFor returns all contracts associated with a symbol
func (o *CompilerOracle) GetContractsFor(symbolName string) []Contract {
	o.mu.RLock()
	defer o.mu.RUnlock()
	var res []Contract
	for _, c := range o.contracts {
		if c.TargetSymbol == symbolName {
			res = append(res, c)
		}
	}
	return res
}
