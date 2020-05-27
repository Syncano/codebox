package common

import (
	"io"

	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

type ChunkReader struct {
	isDirty bool
	getFunc func() (*scriptpb.RunChunk, error)
}

func (cr *ChunkReader) Get() (*scriptpb.RunChunk, error) {
	cr.isDirty = true
	return cr.getFunc()
}

func (cr *ChunkReader) IsDirty() bool {
	return cr.isDirty
}

func NewChunkReader(getFunc func() (*scriptpb.RunChunk, error)) *ChunkReader {
	return &ChunkReader{
		getFunc: getFunc,
	}
}

func NewArrayChunkReader(chunks []*scriptpb.RunChunk) *ChunkReader {
	return &ChunkReader{
		getFunc: func() (*scriptpb.RunChunk, error) {
			if len(chunks) == 0 {
				return nil, io.EOF
			}

			chunk := chunks[0]
			chunks = chunks[1:]

			return chunk, nil
		},
	}
}
