package records

// ChainAnchor 审计链锚点实体（对应 audit_chain_anchors 表）。
type ChainAnchor struct {
	ID             string
	OrganizationID string
	ChainID        string
	AnchorSeq      int64
	EntryHash      []byte
	Proof          string
}

// ChainEntry 审计链条目（chain_seq + entry_hash + prev_hash + chain_algo）。
type ChainEntry struct {
	ChainID   string
	ChainSeq  int64
	PrevHash  []byte
	EntryHash []byte
	ChainAlgo string
}
