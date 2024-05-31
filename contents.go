package mlcrestorerdownloader

type Content struct {
	ID     uint32
	Index  []byte
	Type   uint16
	Size   uint64
	Hash   []byte
	CIDStr string
}
