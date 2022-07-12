package model

type KeyInfo struct {
	Key            string
	Value          string
	Version        int64
	CreateRevision int64
	ModRevision    int64
	Lease          int64
}