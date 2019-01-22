// Code generated by go-bindata.
// sources:
// assets/wrappers/node.js
// DO NOT EDIT!

package assets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

func bindataRead(data, name string) ([]byte, error) {
	var empty [0]byte
	sx := (*reflect.StringHeader)(unsafe.Pointer(&data))
	b := empty[:]
	bx := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bx.Data = sx.Data
	bx.Len = len(data)
	bx.Cap = bx.Len
	return b, nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _wrappersNodeJs = "\x63\x6f\x6e\x73\x74\x20\x66\x73\x20\x3d\x20\x72\x65\x71\x75\x69\x72\x65\x28\x27\x66\x73\x27\x29\x0a\x63\x6f\x6e\x73\x74\x20\x70\x61\x74\x68\x20\x3d\x20\x72\x65\x71\x75\x69\x72\x65\x28\x27\x70\x61\x74\x68\x27\x29\x0a\x63\x6f\x6e\x73\x74\x20\x76\x6d\x20\x3d\x20\x72\x65\x71\x75\x69\x72\x65\x28\x27\x76\x6d\x27\x29\x0a\x63\x6f\x6e\x73\x74\x20\x6e\x65\x74\x20\x3d\x20\x72\x65\x71\x75\x69\x72\x65\x28\x27\x6e\x65\x74\x27\x29\x0a\x63\x6f\x6e\x73\x74\x20\x6f\x73\x20\x3d\x20\x72\x65\x71\x75\x69\x72\x65\x28\x27\x6f\x73\x27\x29\x0a\x0a\x63\x6f\x6e\x73\x74\x20\x41\x50\x50\x5f\x50\x41\x54\x48\x20\x3d\x20\x27\x2f\x61\x70\x70\x2f\x63\x6f\x64\x65\x27\x0a\x63\x6f\x6e\x73\x74\x20\x45\x4e\x56\x5f\x50\x41\x54\x48\x20\x3d\x20\x27\x2f\x61\x70\x70\x2f\x65\x6e\x76\x27\x0a\x63\x6f\x6e\x73\x74\x20\x41\x50\x50\x5f\x50\x4f\x52\x54\x20\x3d\x20\x38\x31\x32\x33\x0a\x63\x6f\x6e\x73\x74\x20\x4d\x55\x58\x5f\x53\x54\x44\x4f\x55\x54\x20\x3d\x20\x31\x0a\x63\x6f\x6e\x73\x74\x20\x4d\x55\x58\x5f\x53\x54\x44\x45\x52\x52\x20\x3d\x20\x32\x0a\x63\x6f\x6e\x73\x74\x20\x4d\x55\x58\x5f\x52\x45\x53\x50\x4f\x4e\x53\x45\x20\x3d\x20\x33\x0a\x63\x6f\x6e\x73\x74\x20\x53\x43\x52\x49\x50\x54\x5f\x46\x55\x4e\x43\x20\x3d\x20\x6e\x65\x77\x20\x76\x6d\x2e\x53\x63\x72\x69\x70\x74\x28\x60\x0a\x7b\x0a\x20\x20\x6c\x65\x74\x20\x5f\x5f\x66\x0a\x0a\x20\x20\x74\x72\x79\x20\x7b\x0a\x20\x20\x20\x20\x5f\x5f\x66\x20\x3d\x20\x5f\x5f\x66\x75\x6e\x63\x28\x7b\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x61\x72\x67\x73\x3a\x20\x41\x52\x47\x53\x2c\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x6d\x65\x74\x61\x3a\x20\x4d\x45\x54\x41\x2c\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x63\x6f\x6e\x66\x69\x67\x3a\x20\x43\x4f\x4e\x46\x49\x47\x2c\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x48\x74\x74\x70\x52\x65\x73\x70\x6f\x6e\x73\x65\x2c\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x73\x65\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x2c\x0a\x20\x20\x20\x20\x7d\x29\x0a\x20\x20\x7d\x20\x63\x61\x74\x63\x68\x20\x28\x65\x72\x72\x6f\x72\x29\x20\x7b\x0a\x20\x20\x20\x20\x5f\x5f\x73\x63\x72\x69\x70\x74\x2e\x68\x61\x6e\x64\x6c\x65\x45\x72\x72\x6f\x72\x28\x65\x72\x72\x6f\x72\x29\x0a\x20\x20\x7d\x0a\x0a\x20\x20\x69\x66\x20\x28\x5f\x5f\x66\x20\x69\x6e\x73\x74\x61\x6e\x63\x65\x6f\x66\x20\x50\x72\x6f\x6d\x69\x73\x65\x29\x20\x7b\x0a\x20\x20\x20\x20\x5f\x5f\x66\x2e\x63\x61\x74\x63\x68\x28\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x28\x65\x72\x72\x6f\x72\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x5f\x5f\x73\x63\x72\x69\x70\x74\x2e\x68\x61\x6e\x64\x6c\x65\x45\x72\x72\x6f\x72\x28\x65\x72\x72\x6f\x72\x29\x0a\x20\x20\x20\x20\x7d\x29\x0a\x0a\x20\x20\x20\x20\x5f\x5f\x66\x2e\x74\x68\x65\x6e\x28\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x28\x72\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x5f\x5f\x73\x63\x72\x69\x70\x74\x2e\x73\x65\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x28\x72\x29\x0a\x20\x20\x20\x20\x20\x20\x5f\x5f\x73\x63\x72\x69\x70\x74\x2e\x73\x65\x6e\x64\x52\x65\x73\x70\x6f\x6e\x73\x65\x28\x29\x0a\x20\x20\x20\x20\x7d\x29\x0a\x0a\x20\x20\x7d\x20\x65\x6c\x73\x65\x20\x7b\x0a\x20\x20\x20\x20\x5f\x5f\x73\x63\x72\x69\x70\x74\x2e\x73\x65\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x28\x5f\x5f\x66\x29\x0a\x20\x20\x20\x20\x5f\x5f\x73\x63\x72\x69\x70\x74\x2e\x73\x65\x6e\x64\x52\x65\x73\x70\x6f\x6e\x73\x65\x28\x29\x0a\x7d\x0a\x7d\x60\x29\x0a\x0a\x6c\x65\x74\x20\x6c\x61\x73\x74\x43\x6f\x6e\x74\x65\x78\x74\x0a\x6c\x65\x74\x20\x73\x63\x72\x69\x70\x74\x0a\x6c\x65\x74\x20\x73\x63\x72\x69\x70\x74\x46\x75\x6e\x63\x0a\x6c\x65\x74\x20\x73\x6f\x63\x6b\x65\x74\x0a\x6c\x65\x74\x20\x64\x61\x74\x61\x0a\x6c\x65\x74\x20\x64\x61\x74\x61\x43\x75\x72\x73\x6f\x72\x0a\x6c\x65\x74\x20\x6f\x72\x69\x67\x53\x74\x64\x6f\x75\x74\x57\x72\x69\x74\x65\x0a\x6c\x65\x74\x20\x61\x73\x79\x6e\x63\x4d\x6f\x64\x65\x0a\x0a\x2f\x2f\x20\x50\x61\x74\x63\x68\x20\x70\x72\x6f\x63\x65\x73\x73\x2e\x65\x78\x69\x74\x2e\x0a\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x45\x78\x69\x74\x45\x72\x72\x6f\x72\x20\x28\x63\x6f\x64\x65\x29\x20\x7b\x0a\x20\x20\x74\x68\x69\x73\x2e\x63\x6f\x64\x65\x20\x3d\x20\x63\x6f\x64\x65\x20\x21\x3d\x3d\x20\x75\x6e\x64\x65\x66\x69\x6e\x65\x64\x20\x3f\x20\x63\x6f\x64\x65\x20\x3a\x20\x70\x72\x6f\x63\x65\x73\x73\x2e\x65\x78\x69\x74\x43\x6f\x64\x65\x0a\x7d\x0a\x0a\x45\x78\x69\x74\x45\x72\x72\x6f\x72\x2e\x70\x72\x6f\x74\x6f\x74\x79\x70\x65\x20\x3d\x20\x6e\x65\x77\x20\x45\x72\x72\x6f\x72\x28\x29\x0a\x70\x72\x6f\x63\x65\x73\x73\x2e\x65\x78\x69\x74\x20\x3d\x20\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x28\x63\x6f\x64\x65\x29\x20\x7b\x0a\x20\x20\x74\x68\x72\x6f\x77\x20\x6e\x65\x77\x20\x45\x78\x69\x74\x45\x72\x72\x6f\x72\x28\x63\x6f\x64\x65\x29\x0a\x7d\x0a\x0a\x2f\x2f\x20\x50\x61\x74\x63\x68\x20\x70\x72\x6f\x63\x65\x73\x73\x20\x73\x74\x64\x6f\x75\x74\x2f\x73\x74\x64\x65\x72\x72\x20\x77\x72\x69\x74\x65\x2e\x0a\x6f\x72\x69\x67\x53\x74\x64\x6f\x75\x74\x57\x72\x69\x74\x65\x20\x3d\x20\x70\x72\x6f\x63\x65\x73\x73\x2e\x73\x74\x64\x6f\x75\x74\x2e\x77\x72\x69\x74\x65\x0a\x70\x72\x6f\x63\x65\x73\x73\x2e\x73\x74\x64\x6f\x75\x74\x2e\x77\x72\x69\x74\x65\x20\x3d\x20\x73\x74\x72\x65\x61\x6d\x57\x72\x69\x74\x65\x28\x4d\x55\x58\x5f\x53\x54\x44\x4f\x55\x54\x29\x0a\x70\x72\x6f\x63\x65\x73\x73\x2e\x73\x74\x64\x65\x72\x72\x2e\x77\x72\x69\x74\x65\x20\x3d\x20\x73\x74\x72\x65\x61\x6d\x57\x72\x69\x74\x65\x28\x4d\x55\x58\x5f\x53\x54\x44\x45\x52\x52\x29\x0a\x0a\x6c\x65\x74\x20\x63\x6f\x6d\x6d\x6f\x6e\x43\x74\x78\x20\x3d\x20\x7b\x0a\x20\x20\x41\x50\x50\x5f\x50\x41\x54\x48\x2c\x0a\x20\x20\x45\x4e\x56\x5f\x50\x41\x54\x48\x2c\x0a\x20\x20\x5f\x5f\x66\x69\x6c\x65\x6e\x61\x6d\x65\x2c\x0a\x20\x20\x5f\x5f\x64\x69\x72\x6e\x61\x6d\x65\x2c\x0a\x0a\x20\x20\x2f\x2f\x20\x67\x6c\x6f\x62\x61\x6c\x73\x0a\x20\x20\x65\x78\x70\x6f\x72\x74\x73\x2c\x0a\x20\x20\x70\x72\x6f\x63\x65\x73\x73\x2c\x0a\x20\x20\x42\x75\x66\x66\x65\x72\x2c\x0a\x20\x20\x63\x6c\x65\x61\x72\x49\x6d\x6d\x65\x64\x69\x61\x74\x65\x2c\x0a\x20\x20\x63\x6c\x65\x61\x72\x49\x6e\x74\x65\x72\x76\x61\x6c\x2c\x0a\x20\x20\x63\x6c\x65\x61\x72\x54\x69\x6d\x65\x6f\x75\x74\x2c\x0a\x20\x20\x73\x65\x74\x49\x6d\x6d\x65\x64\x69\x61\x74\x65\x2c\x0a\x20\x20\x73\x65\x74\x49\x6e\x74\x65\x72\x76\x61\x6c\x2c\x0a\x20\x20\x73\x65\x74\x54\x69\x6d\x65\x6f\x75\x74\x2c\x0a\x20\x20\x63\x6f\x6e\x73\x6f\x6c\x65\x2c\x0a\x20\x20\x6d\x6f\x64\x75\x6c\x65\x2c\x0a\x20\x20\x72\x65\x71\x75\x69\x72\x65\x0a\x7d\x0a\x0a\x2f\x2f\x20\x49\x6e\x6a\x65\x63\x74\x20\x67\x6c\x6f\x62\x61\x6c\x73\x20\x74\x6f\x20\x63\x6f\x6d\x6d\x6f\x6e\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x0a\x63\x6f\x6d\x6d\x6f\x6e\x43\x74\x78\x5b\x27\x67\x6c\x6f\x62\x61\x6c\x27\x5d\x20\x3d\x20\x63\x6f\x6d\x6d\x6f\x6e\x43\x74\x78\x0a\x0a\x2f\x2f\x20\x48\x74\x74\x70\x52\x65\x73\x70\x6f\x6e\x73\x65\x20\x63\x6c\x61\x73\x73\x2e\x0a\x63\x6c\x61\x73\x73\x20\x48\x74\x74\x70\x52\x65\x73\x70\x6f\x6e\x73\x65\x20\x7b\x0a\x20\x20\x63\x6f\x6e\x73\x74\x72\x75\x63\x74\x6f\x72\x20\x28\x73\x74\x61\x74\x75\x73\x43\x6f\x64\x65\x2c\x20\x63\x6f\x6e\x74\x65\x6e\x74\x2c\x20\x63\x6f\x6e\x74\x65\x6e\x74\x54\x79\x70\x65\x2c\x20\x68\x65\x61\x64\x65\x72\x73\x29\x20\x7b\x0a\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x73\x74\x61\x74\x75\x73\x43\x6f\x64\x65\x20\x3d\x20\x73\x74\x61\x74\x75\x73\x43\x6f\x64\x65\x20\x7c\x7c\x20\x32\x30\x30\x0a\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x63\x6f\x6e\x74\x65\x6e\x74\x20\x3d\x20\x63\x6f\x6e\x74\x65\x6e\x74\x20\x7c\x7c\x20\x27\x27\x0a\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x63\x6f\x6e\x74\x65\x6e\x74\x54\x79\x70\x65\x20\x3d\x20\x63\x6f\x6e\x74\x65\x6e\x74\x54\x79\x70\x65\x20\x7c\x7c\x20\x27\x61\x70\x70\x6c\x69\x63\x61\x74\x69\x6f\x6e\x2f\x6a\x73\x6f\x6e\x27\x0a\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x68\x65\x61\x64\x65\x72\x73\x20\x3d\x20\x68\x65\x61\x64\x65\x72\x73\x20\x7c\x7c\x20\x7b\x7d\x0a\x20\x20\x7d\x0a\x0a\x20\x20\x6a\x73\x6f\x6e\x20\x28\x29\x20\x7b\x0a\x20\x20\x20\x20\x66\x6f\x72\x20\x28\x6c\x65\x74\x20\x6b\x20\x69\x6e\x20\x74\x68\x69\x73\x2e\x68\x65\x61\x64\x65\x72\x73\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x68\x65\x61\x64\x65\x72\x73\x5b\x6b\x5d\x20\x3d\x20\x53\x74\x72\x69\x6e\x67\x28\x74\x68\x69\x73\x2e\x68\x65\x61\x64\x65\x72\x73\x5b\x6b\x5d\x29\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x20\x20\x72\x65\x74\x75\x72\x6e\x20\x4a\x53\x4f\x4e\x2e\x73\x74\x72\x69\x6e\x67\x69\x66\x79\x28\x7b\x0a\x20\x20\x20\x20\x20\x20\x73\x63\x3a\x20\x70\x61\x72\x73\x65\x49\x6e\x74\x28\x74\x68\x69\x73\x2e\x73\x74\x61\x74\x75\x73\x43\x6f\x64\x65\x29\x2c\x0a\x20\x20\x20\x20\x20\x20\x63\x74\x3a\x20\x53\x74\x72\x69\x6e\x67\x28\x74\x68\x69\x73\x2e\x63\x6f\x6e\x74\x65\x6e\x74\x54\x79\x70\x65\x29\x2c\x0a\x20\x20\x20\x20\x20\x20\x68\x3a\x20\x74\x68\x69\x73\x2e\x68\x65\x61\x64\x65\x72\x73\x0a\x20\x20\x20\x20\x7d\x29\x0a\x20\x20\x7d\x0a\x7d\x0a\x63\x6f\x6d\x6d\x6f\x6e\x43\x74\x78\x5b\x27\x48\x74\x74\x70\x52\x65\x73\x70\x6f\x6e\x73\x65\x27\x5d\x20\x3d\x20\x48\x74\x74\x70\x52\x65\x73\x70\x6f\x6e\x73\x65\x0a\x0a\x2f\x2f\x20\x53\x63\x72\x69\x70\x74\x43\x6f\x6e\x74\x65\x78\x74\x20\x63\x6c\x61\x73\x73\x2e\x0a\x63\x6c\x61\x73\x73\x20\x53\x63\x72\x69\x70\x74\x43\x6f\x6e\x74\x65\x78\x74\x20\x7b\x0a\x20\x20\x63\x6f\x6e\x73\x74\x72\x75\x63\x74\x6f\x72\x20\x28\x72\x65\x73\x70\x6f\x6e\x73\x65\x4d\x75\x78\x20\x3d\x20\x4d\x55\x58\x5f\x52\x45\x53\x50\x4f\x4e\x53\x45\x29\x20\x7b\x0a\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x6f\x75\x74\x70\x75\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x20\x3d\x20\x6e\x75\x6c\x6c\x0a\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x65\x78\x69\x74\x43\x6f\x64\x65\x20\x3d\x20\x6e\x75\x6c\x6c\x0a\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x72\x65\x73\x70\x6f\x6e\x73\x65\x4d\x75\x78\x20\x3d\x20\x72\x65\x73\x70\x6f\x6e\x73\x65\x4d\x75\x78\x0a\x20\x20\x7d\x0a\x0a\x20\x20\x73\x65\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x20\x28\x72\x65\x73\x70\x6f\x6e\x73\x65\x29\x20\x7b\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x72\x65\x73\x70\x6f\x6e\x73\x65\x20\x69\x6e\x73\x74\x61\x6e\x63\x65\x6f\x66\x20\x48\x74\x74\x70\x52\x65\x73\x70\x6f\x6e\x73\x65\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x6f\x75\x74\x70\x75\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x20\x3d\x20\x72\x65\x73\x70\x6f\x6e\x73\x65\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x7d\x0a\x0a\x20\x20\x68\x61\x6e\x64\x6c\x65\x45\x72\x72\x6f\x72\x20\x28\x65\x72\x72\x6f\x72\x29\x20\x7b\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x65\x72\x72\x6f\x72\x20\x69\x6e\x73\x74\x61\x6e\x63\x65\x6f\x66\x20\x45\x78\x69\x74\x45\x72\x72\x6f\x72\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x65\x78\x69\x74\x43\x6f\x64\x65\x20\x3d\x20\x65\x72\x72\x6f\x72\x2e\x63\x6f\x64\x65\x0a\x20\x20\x20\x20\x20\x20\x72\x65\x74\x75\x72\x6e\x0a\x20\x20\x20\x20\x7d\x0a\x0a\x20\x20\x20\x20\x63\x6f\x6e\x73\x6f\x6c\x65\x2e\x65\x72\x72\x6f\x72\x28\x65\x72\x72\x6f\x72\x29\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x65\x72\x72\x6f\x72\x2e\x6d\x65\x73\x73\x61\x67\x65\x20\x3d\x3d\x3d\x20\x27\x53\x63\x72\x69\x70\x74\x20\x65\x78\x65\x63\x75\x74\x69\x6f\x6e\x20\x74\x69\x6d\x65\x64\x20\x6f\x75\x74\x2e\x27\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x65\x78\x69\x74\x43\x6f\x64\x65\x20\x3d\x20\x31\x32\x34\x0a\x20\x20\x20\x20\x7d\x20\x65\x6c\x73\x65\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x74\x68\x69\x73\x2e\x65\x78\x69\x74\x43\x6f\x64\x65\x20\x3d\x20\x31\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x7d\x0a\x0a\x20\x20\x73\x65\x6e\x64\x52\x65\x73\x70\x6f\x6e\x73\x65\x20\x28\x66\x69\x6e\x61\x6c\x20\x3d\x20\x66\x61\x6c\x73\x65\x29\x20\x7b\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x21\x66\x69\x6e\x61\x6c\x20\x26\x26\x20\x21\x61\x73\x79\x6e\x63\x4d\x6f\x64\x65\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x72\x65\x74\x75\x72\x6e\x0a\x20\x20\x20\x20\x7d\x0a\x0a\x20\x20\x20\x20\x6c\x65\x74\x20\x65\x78\x69\x74\x43\x6f\x64\x65\x20\x3d\x20\x70\x72\x6f\x63\x65\x73\x73\x2e\x65\x78\x69\x74\x43\x6f\x64\x65\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x74\x68\x69\x73\x2e\x65\x78\x69\x74\x43\x6f\x64\x65\x20\x21\x3d\x3d\x20\x6e\x75\x6c\x6c\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x65\x78\x69\x74\x43\x6f\x64\x65\x20\x3d\x20\x74\x68\x69\x73\x2e\x65\x78\x69\x74\x43\x6f\x64\x65\x0a\x20\x20\x20\x20\x7d\x0a\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x74\x68\x69\x73\x2e\x6f\x75\x74\x70\x75\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x20\x21\x3d\x3d\x20\x6e\x75\x6c\x6c\x20\x26\x26\x20\x74\x68\x69\x73\x2e\x6f\x75\x74\x70\x75\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x20\x69\x6e\x73\x74\x61\x6e\x63\x65\x6f\x66\x20\x48\x74\x74\x70\x52\x65\x73\x70\x6f\x6e\x73\x65\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x6c\x65\x74\x20\x6a\x73\x6f\x6e\x20\x3d\x20\x74\x68\x69\x73\x2e\x6f\x75\x74\x70\x75\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x2e\x6a\x73\x6f\x6e\x28\x29\x0a\x20\x20\x20\x20\x20\x20\x6c\x65\x74\x20\x6a\x73\x6f\x6e\x4c\x65\x6e\x20\x3d\x20\x42\x75\x66\x66\x65\x72\x2e\x61\x6c\x6c\x6f\x63\x55\x6e\x73\x61\x66\x65\x28\x34\x29\x0a\x20\x20\x20\x20\x20\x20\x6a\x73\x6f\x6e\x4c\x65\x6e\x2e\x77\x72\x69\x74\x65\x55\x49\x6e\x74\x33\x32\x4c\x45\x28\x42\x75\x66\x66\x65\x72\x2e\x62\x79\x74\x65\x4c\x65\x6e\x67\x74\x68\x28\x6a\x73\x6f\x6e\x29\x29\x0a\x20\x20\x20\x20\x20\x20\x73\x6f\x63\x6b\x65\x74\x57\x72\x69\x74\x65\x28\x73\x6f\x63\x6b\x65\x74\x2c\x20\x74\x68\x69\x73\x2e\x72\x65\x73\x70\x6f\x6e\x73\x65\x4d\x75\x78\x2c\x20\x53\x74\x72\x69\x6e\x67\x2e\x66\x72\x6f\x6d\x43\x68\x61\x72\x43\x6f\x64\x65\x28\x65\x78\x69\x74\x43\x6f\x64\x65\x29\x2c\x20\x6a\x73\x6f\x6e\x4c\x65\x6e\x2c\x20\x6a\x73\x6f\x6e\x2c\x20\x74\x68\x69\x73\x2e\x6f\x75\x74\x70\x75\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x2e\x63\x6f\x6e\x74\x65\x6e\x74\x29\x0a\x20\x20\x20\x20\x7d\x20\x65\x6c\x73\x65\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x73\x6f\x63\x6b\x65\x74\x57\x72\x69\x74\x65\x28\x73\x6f\x63\x6b\x65\x74\x2c\x20\x74\x68\x69\x73\x2e\x72\x65\x73\x70\x6f\x6e\x73\x65\x4d\x75\x78\x2c\x20\x53\x74\x72\x69\x6e\x67\x2e\x66\x72\x6f\x6d\x43\x68\x61\x72\x43\x6f\x64\x65\x28\x65\x78\x69\x74\x43\x6f\x64\x65\x29\x29\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x7d\x0a\x7d\x0a\x0a\x2f\x2f\x20\x4d\x61\x69\x6e\x20\x70\x72\x6f\x63\x65\x73\x73\x20\x73\x63\x72\x69\x70\x74\x20\x66\x75\x6e\x63\x74\x69\x6f\x6e\x2e\x0a\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x70\x72\x6f\x63\x65\x73\x73\x53\x63\x72\x69\x70\x74\x20\x28\x63\x6f\x6e\x74\x65\x78\x74\x29\x20\x7b\x0a\x20\x20\x63\x6f\x6e\x73\x74\x20\x74\x69\x6d\x65\x6f\x75\x74\x20\x3d\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x5f\x74\x69\x6d\x65\x6f\x75\x74\x0a\x0a\x20\x20\x2f\x2f\x20\x43\x72\x65\x61\x74\x65\x20\x73\x63\x72\x69\x70\x74\x20\x61\x6e\x64\x20\x63\x6f\x6e\x74\x65\x78\x74\x20\x69\x66\x20\x69\x74\x27\x73\x20\x74\x68\x65\x20\x66\x69\x72\x73\x74\x20\x72\x75\x6e\x2e\x0a\x20\x20\x69\x66\x20\x28\x73\x63\x72\x69\x70\x74\x20\x3d\x3d\x3d\x20\x75\x6e\x64\x65\x66\x69\x6e\x65\x64\x29\x20\x7b\x0a\x20\x20\x20\x20\x2f\x2f\x20\x53\x65\x74\x75\x70\x20\x61\x73\x79\x6e\x63\x20\x6d\x6f\x64\x65\x2e\x0a\x20\x20\x20\x20\x73\x65\x74\x75\x70\x41\x73\x79\x6e\x63\x4d\x6f\x64\x65\x28\x63\x6f\x6e\x74\x65\x78\x74\x2e\x5f\x61\x73\x79\x6e\x63\x29\x0a\x0a\x20\x20\x20\x20\x63\x6f\x6e\x73\x74\x20\x65\x6e\x74\x72\x79\x50\x6f\x69\x6e\x74\x20\x3d\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x5f\x65\x6e\x74\x72\x79\x50\x6f\x69\x6e\x74\x0a\x20\x20\x20\x20\x63\x6f\x6e\x73\x74\x20\x73\x63\x72\x69\x70\x74\x46\x69\x6c\x65\x6e\x61\x6d\x65\x20\x3d\x20\x70\x61\x74\x68\x2e\x6a\x6f\x69\x6e\x28\x41\x50\x50\x5f\x50\x41\x54\x48\x2c\x20\x65\x6e\x74\x72\x79\x50\x6f\x69\x6e\x74\x29\x0a\x20\x20\x20\x20\x63\x6f\x6e\x73\x74\x20\x73\x6f\x75\x72\x63\x65\x20\x3d\x20\x66\x73\x2e\x72\x65\x61\x64\x46\x69\x6c\x65\x53\x79\x6e\x63\x28\x73\x63\x72\x69\x70\x74\x46\x69\x6c\x65\x6e\x61\x6d\x65\x29\x0a\x0a\x20\x20\x20\x20\x63\x6f\x6d\x6d\x6f\x6e\x43\x74\x78\x2e\x6d\x6f\x64\x75\x6c\x65\x2e\x66\x69\x6c\x65\x6e\x61\x6d\x65\x20\x3d\x20\x73\x63\x72\x69\x70\x74\x46\x69\x6c\x65\x6e\x61\x6d\x65\x0a\x20\x20\x20\x20\x63\x6f\x6d\x6d\x6f\x6e\x43\x74\x78\x2e\x5f\x5f\x66\x69\x6c\x65\x6e\x61\x6d\x65\x20\x3d\x20\x73\x63\x72\x69\x70\x74\x46\x69\x6c\x65\x6e\x61\x6d\x65\x0a\x20\x20\x20\x20\x63\x6f\x6d\x6d\x6f\x6e\x43\x74\x78\x2e\x5f\x5f\x64\x69\x72\x6e\x61\x6d\x65\x20\x3d\x20\x70\x61\x74\x68\x2e\x64\x69\x72\x6e\x61\x6d\x65\x28\x73\x63\x72\x69\x70\x74\x46\x69\x6c\x65\x6e\x61\x6d\x65\x29\x0a\x0a\x20\x20\x20\x20\x73\x63\x72\x69\x70\x74\x20\x3d\x20\x6e\x65\x77\x20\x76\x6d\x2e\x53\x63\x72\x69\x70\x74\x28\x73\x6f\x75\x72\x63\x65\x2c\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x66\x69\x6c\x65\x6e\x61\x6d\x65\x3a\x20\x65\x6e\x74\x72\x79\x50\x6f\x69\x6e\x74\x0a\x20\x20\x20\x20\x7d\x29\x0a\x20\x20\x7d\x0a\x0a\x20\x20\x2f\x2f\x20\x50\x72\x65\x70\x61\x72\x65\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x0a\x20\x20\x6c\x65\x74\x20\x63\x74\x78\x20\x3d\x20\x4f\x62\x6a\x65\x63\x74\x2e\x61\x73\x73\x69\x67\x6e\x28\x7b\x7d\x2c\x20\x63\x6f\x6d\x6d\x6f\x6e\x43\x74\x78\x29\x0a\x0a\x20\x20\x66\x6f\x72\x20\x28\x6c\x65\x74\x20\x6b\x65\x79\x20\x69\x6e\x20\x63\x6f\x6e\x74\x65\x78\x74\x29\x20\x7b\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x6b\x65\x79\x2e\x73\x74\x61\x72\x74\x73\x57\x69\x74\x68\x28\x27\x5f\x27\x29\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x63\x6f\x6e\x74\x69\x6e\x75\x65\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x20\x20\x63\x74\x78\x5b\x6b\x65\x79\x5d\x20\x3d\x20\x63\x6f\x6e\x74\x65\x78\x74\x5b\x6b\x65\x79\x5d\x0a\x20\x20\x7d\x0a\x20\x20\x63\x74\x78\x5b\x27\x5f\x5f\x73\x63\x72\x69\x70\x74\x27\x5d\x20\x3d\x20\x6c\x61\x73\x74\x43\x6f\x6e\x74\x65\x78\x74\x20\x3d\x20\x6e\x65\x77\x20\x53\x63\x72\x69\x70\x74\x43\x6f\x6e\x74\x65\x78\x74\x28\x63\x74\x78\x2e\x5f\x72\x65\x73\x70\x6f\x6e\x73\x65\x29\x0a\x20\x20\x2f\x2f\x20\x46\x6f\x72\x20\x62\x61\x63\x6b\x77\x61\x72\x64\x73\x20\x63\x6f\x6d\x70\x61\x74\x69\x62\x69\x6c\x69\x74\x79\x2e\x0a\x20\x20\x63\x74\x78\x5b\x27\x73\x65\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x27\x5d\x20\x3d\x20\x28\x72\x29\x20\x3d\x3e\x20\x6c\x61\x73\x74\x43\x6f\x6e\x74\x65\x78\x74\x2e\x73\x65\x74\x52\x65\x73\x70\x6f\x6e\x73\x65\x28\x72\x29\x0a\x0a\x20\x20\x2f\x2f\x20\x52\x75\x6e\x20\x73\x63\x72\x69\x70\x74\x2e\x0a\x20\x20\x6c\x65\x74\x20\x6f\x70\x74\x73\x20\x3d\x20\x7b\x20\x74\x69\x6d\x65\x6f\x75\x74\x3a\x20\x74\x69\x6d\x65\x6f\x75\x74\x20\x2f\x20\x31\x65\x36\x20\x7d\x0a\x20\x20\x69\x66\x20\x28\x73\x63\x72\x69\x70\x74\x46\x75\x6e\x63\x20\x3d\x3d\x3d\x20\x75\x6e\x64\x65\x66\x69\x6e\x65\x64\x29\x20\x7b\x0a\x20\x20\x20\x20\x6c\x65\x74\x20\x72\x65\x74\x20\x3d\x20\x73\x63\x72\x69\x70\x74\x2e\x72\x75\x6e\x49\x6e\x4e\x65\x77\x43\x6f\x6e\x74\x65\x78\x74\x28\x63\x74\x78\x2c\x20\x6f\x70\x74\x73\x29\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x74\x79\x70\x65\x6f\x66\x20\x28\x72\x65\x74\x29\x20\x3d\x3d\x3d\x20\x27\x66\x75\x6e\x63\x74\x69\x6f\x6e\x27\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x73\x63\x72\x69\x70\x74\x46\x75\x6e\x63\x20\x3d\x20\x72\x65\x74\x0a\x20\x20\x20\x20\x20\x20\x63\x74\x78\x5b\x27\x5f\x5f\x66\x75\x6e\x63\x27\x5d\x20\x3d\x20\x63\x6f\x6d\x6d\x6f\x6e\x43\x74\x78\x5b\x27\x5f\x5f\x66\x75\x6e\x63\x27\x5d\x20\x3d\x20\x73\x63\x72\x69\x70\x74\x46\x75\x6e\x63\x0a\x20\x20\x20\x20\x20\x20\x53\x43\x52\x49\x50\x54\x5f\x46\x55\x4e\x43\x2e\x72\x75\x6e\x49\x6e\x4e\x65\x77\x43\x6f\x6e\x74\x65\x78\x74\x28\x63\x74\x78\x2c\x20\x6f\x70\x74\x73\x29\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x7d\x20\x65\x6c\x73\x65\x20\x7b\x0a\x20\x20\x20\x20\x2f\x2f\x20\x52\x75\x6e\x20\x73\x63\x72\x69\x70\x74\x20\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x69\x66\x20\x69\x74\x27\x73\x20\x64\x65\x66\x69\x6e\x65\x64\x2e\x0a\x20\x20\x20\x20\x53\x43\x52\x49\x50\x54\x5f\x46\x55\x4e\x43\x2e\x72\x75\x6e\x49\x6e\x4e\x65\x77\x43\x6f\x6e\x74\x65\x78\x74\x28\x63\x74\x78\x2c\x20\x6f\x70\x74\x73\x29\x0a\x20\x20\x7d\x0a\x7d\x0a\x0a\x2f\x2f\x20\x43\x72\x65\x61\x74\x65\x20\x73\x65\x72\x76\x65\x72\x20\x61\x6e\x64\x20\x70\x72\x6f\x63\x65\x73\x73\x20\x64\x61\x74\x61\x20\x6f\x6e\x20\x69\x74\x2e\x0a\x76\x61\x72\x20\x73\x65\x72\x76\x65\x72\x20\x3d\x20\x6e\x65\x74\x2e\x63\x72\x65\x61\x74\x65\x53\x65\x72\x76\x65\x72\x28\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x28\x73\x6f\x63\x6b\x29\x20\x7b\x0a\x20\x20\x73\x6f\x63\x6b\x65\x74\x20\x3d\x20\x73\x6f\x63\x6b\x0a\x0a\x20\x20\x73\x6f\x63\x6b\x2e\x6f\x6e\x28\x27\x64\x61\x74\x61\x27\x2c\x20\x28\x63\x68\x75\x6e\x6b\x29\x20\x3d\x3e\x20\x7b\x0a\x20\x20\x20\x20\x73\x6f\x63\x6b\x2e\x75\x6e\x72\x65\x66\x28\x29\x0a\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x64\x61\x74\x61\x20\x3d\x3d\x3d\x20\x75\x6e\x64\x65\x66\x69\x6e\x65\x64\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x6c\x65\x74\x20\x74\x6f\x74\x61\x6c\x53\x69\x7a\x65\x20\x3d\x20\x63\x68\x75\x6e\x6b\x2e\x72\x65\x61\x64\x55\x49\x6e\x74\x33\x32\x4c\x45\x28\x30\x29\x0a\x20\x20\x20\x20\x20\x20\x64\x61\x74\x61\x43\x75\x72\x73\x6f\x72\x20\x3d\x20\x30\x0a\x20\x20\x20\x20\x20\x20\x64\x61\x74\x61\x20\x3d\x20\x42\x75\x66\x66\x65\x72\x2e\x61\x6c\x6c\x6f\x63\x55\x6e\x73\x61\x66\x65\x28\x74\x6f\x74\x61\x6c\x53\x69\x7a\x65\x29\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x20\x20\x63\x68\x75\x6e\x6b\x2e\x63\x6f\x70\x79\x28\x64\x61\x74\x61\x2c\x20\x64\x61\x74\x61\x43\x75\x72\x73\x6f\x72\x29\x0a\x20\x20\x20\x20\x64\x61\x74\x61\x43\x75\x72\x73\x6f\x72\x20\x2b\x3d\x20\x63\x68\x75\x6e\x6b\x2e\x6c\x65\x6e\x67\x74\x68\x0a\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x64\x61\x74\x61\x43\x75\x72\x73\x6f\x72\x20\x21\x3d\x3d\x20\x64\x61\x74\x61\x2e\x6c\x65\x6e\x67\x74\x68\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x72\x65\x74\x75\x72\x6e\x0a\x20\x20\x20\x20\x7d\x0a\x0a\x20\x20\x20\x20\x2f\x2f\x20\x52\x65\x61\x64\x20\x63\x6f\x6e\x74\x65\x78\x74\x20\x4a\x53\x4f\x4e\x2e\x0a\x20\x20\x20\x20\x6c\x65\x74\x20\x63\x6f\x6e\x74\x65\x78\x74\x53\x69\x7a\x65\x20\x3d\x20\x64\x61\x74\x61\x2e\x72\x65\x61\x64\x55\x49\x6e\x74\x33\x32\x4c\x45\x28\x34\x29\x0a\x20\x20\x20\x20\x6c\x65\x74\x20\x63\x6f\x6e\x74\x65\x78\x74\x4a\x53\x4f\x4e\x20\x3d\x20\x64\x61\x74\x61\x2e\x73\x6c\x69\x63\x65\x28\x38\x2c\x20\x38\x20\x2b\x20\x63\x6f\x6e\x74\x65\x78\x74\x53\x69\x7a\x65\x29\x0a\x20\x20\x20\x20\x6c\x65\x74\x20\x63\x6f\x6e\x74\x65\x78\x74\x20\x3d\x20\x4a\x53\x4f\x4e\x2e\x70\x61\x72\x73\x65\x28\x63\x6f\x6e\x74\x65\x78\x74\x4a\x53\x4f\x4e\x29\x0a\x0a\x20\x20\x20\x20\x2f\x2f\x20\x50\x72\x6f\x63\x65\x73\x73\x20\x66\x69\x6c\x65\x73\x20\x69\x6e\x74\x6f\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x41\x52\x47\x53\x2e\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x63\x6f\x6e\x74\x65\x78\x74\x2e\x5f\x66\x69\x6c\x65\x73\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x41\x52\x47\x53\x20\x3d\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x41\x52\x47\x53\x20\x7c\x7c\x20\x7b\x7d\x0a\x20\x20\x20\x20\x20\x20\x6c\x65\x74\x20\x63\x75\x72\x73\x6f\x72\x20\x3d\x20\x38\x20\x2b\x20\x63\x6f\x6e\x74\x65\x78\x74\x53\x69\x7a\x65\x0a\x0a\x20\x20\x20\x20\x20\x20\x66\x6f\x72\x20\x28\x6c\x65\x74\x20\x69\x20\x3d\x20\x30\x3b\x20\x69\x20\x3c\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x5f\x66\x69\x6c\x65\x73\x2e\x6c\x65\x6e\x67\x74\x68\x3b\x20\x69\x2b\x2b\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x6c\x65\x74\x20\x66\x69\x6c\x65\x20\x3d\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x5f\x66\x69\x6c\x65\x73\x5b\x69\x5d\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x6c\x65\x74\x20\x62\x75\x66\x20\x3d\x20\x64\x61\x74\x61\x2e\x73\x6c\x69\x63\x65\x28\x63\x75\x72\x73\x6f\x72\x2c\x20\x63\x75\x72\x73\x6f\x72\x20\x2b\x20\x66\x69\x6c\x65\x2e\x6c\x65\x6e\x67\x74\x68\x29\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x62\x75\x66\x2e\x63\x6f\x6e\x74\x65\x6e\x74\x54\x79\x70\x65\x20\x3d\x20\x66\x69\x6c\x65\x2e\x63\x74\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x62\x75\x66\x2e\x66\x69\x6c\x65\x6e\x61\x6d\x65\x20\x3d\x20\x66\x69\x6c\x65\x2e\x66\x6e\x61\x6d\x65\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x63\x6f\x6e\x74\x65\x78\x74\x2e\x41\x52\x47\x53\x5b\x66\x69\x6c\x65\x2e\x6e\x61\x6d\x65\x5d\x20\x3d\x20\x62\x75\x66\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x63\x75\x72\x73\x6f\x72\x20\x2b\x3d\x20\x66\x69\x6c\x65\x2e\x6c\x65\x6e\x67\x74\x68\x0a\x20\x20\x20\x20\x20\x20\x7d\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x20\x20\x2f\x2f\x20\x43\x6c\x65\x61\x72\x20\x64\x61\x74\x61\x20\x66\x6f\x72\x20\x6e\x65\x78\x74\x20\x72\x65\x71\x75\x65\x73\x74\x2e\x0a\x20\x20\x20\x20\x64\x61\x74\x61\x20\x3d\x20\x75\x6e\x64\x65\x66\x69\x6e\x65\x64\x0a\x0a\x20\x20\x20\x20\x74\x72\x79\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x70\x72\x6f\x63\x65\x73\x73\x53\x63\x72\x69\x70\x74\x28\x63\x6f\x6e\x74\x65\x78\x74\x29\x0a\x20\x20\x20\x20\x7d\x20\x66\x69\x6e\x61\x6c\x6c\x79\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x69\x66\x20\x28\x21\x61\x73\x79\x6e\x63\x4d\x6f\x64\x65\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x20\x20\x73\x65\x72\x76\x65\x72\x2e\x75\x6e\x72\x65\x66\x28\x29\x0a\x20\x20\x20\x20\x20\x20\x7d\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x7d\x29\x0a\x0a\x20\x20\x73\x6f\x63\x6b\x2e\x6f\x6e\x28\x27\x63\x6c\x6f\x73\x65\x27\x2c\x20\x28\x29\x20\x3d\x3e\x20\x7b\x20\x73\x6f\x63\x6b\x65\x74\x20\x3d\x20\x75\x6e\x64\x65\x66\x69\x6e\x65\x64\x20\x7d\x29\x0a\x7d\x29\x0a\x0a\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x73\x6f\x63\x6b\x65\x74\x57\x72\x69\x74\x65\x20\x28\x73\x6f\x63\x6b\x2c\x20\x6d\x75\x78\x2c\x20\x2e\x2e\x2e\x63\x68\x75\x6e\x6b\x73\x29\x20\x7b\x0a\x20\x20\x69\x66\x20\x28\x73\x6f\x63\x6b\x20\x3d\x3d\x3d\x20\x75\x6e\x64\x65\x66\x69\x6e\x65\x64\x29\x20\x7b\x0a\x20\x20\x20\x20\x72\x65\x74\x75\x72\x6e\x0a\x20\x20\x7d\x0a\x0a\x20\x20\x6c\x65\x74\x20\x74\x6f\x74\x61\x6c\x4c\x65\x6e\x67\x74\x68\x20\x3d\x20\x30\x0a\x20\x20\x66\x6f\x72\x20\x28\x6c\x65\x74\x20\x69\x20\x3d\x20\x30\x3b\x20\x69\x20\x3c\x20\x63\x68\x75\x6e\x6b\x73\x2e\x6c\x65\x6e\x67\x74\x68\x3b\x20\x69\x2b\x2b\x29\x20\x7b\x0a\x20\x20\x20\x20\x74\x6f\x74\x61\x6c\x4c\x65\x6e\x67\x74\x68\x20\x2b\x3d\x20\x42\x75\x66\x66\x65\x72\x2e\x62\x79\x74\x65\x4c\x65\x6e\x67\x74\x68\x28\x63\x68\x75\x6e\x6b\x73\x5b\x69\x5d\x29\x0a\x20\x20\x7d\x0a\x0a\x20\x20\x69\x66\x20\x28\x74\x6f\x74\x61\x6c\x4c\x65\x6e\x67\x74\x68\x20\x3d\x3d\x3d\x20\x30\x29\x20\x7b\x0a\x20\x20\x20\x20\x72\x65\x74\x75\x72\x6e\x0a\x20\x20\x7d\x0a\x0a\x20\x20\x6c\x65\x74\x20\x68\x65\x61\x64\x65\x72\x20\x3d\x20\x42\x75\x66\x66\x65\x72\x2e\x61\x6c\x6c\x6f\x63\x55\x6e\x73\x61\x66\x65\x28\x35\x29\x0a\x20\x20\x68\x65\x61\x64\x65\x72\x2e\x77\x72\x69\x74\x65\x55\x49\x6e\x74\x38\x28\x6d\x75\x78\x29\x0a\x20\x20\x68\x65\x61\x64\x65\x72\x2e\x77\x72\x69\x74\x65\x55\x49\x6e\x74\x33\x32\x4c\x45\x28\x74\x6f\x74\x61\x6c\x4c\x65\x6e\x67\x74\x68\x2c\x20\x31\x29\x0a\x20\x20\x73\x6f\x63\x6b\x2e\x77\x72\x69\x74\x65\x28\x68\x65\x61\x64\x65\x72\x29\x0a\x0a\x20\x20\x66\x6f\x72\x20\x28\x6c\x65\x74\x20\x69\x20\x3d\x20\x30\x3b\x20\x69\x20\x3c\x20\x63\x68\x75\x6e\x6b\x73\x2e\x6c\x65\x6e\x67\x74\x68\x3b\x20\x69\x2b\x2b\x29\x20\x7b\x0a\x20\x20\x20\x20\x73\x6f\x63\x6b\x2e\x77\x72\x69\x74\x65\x28\x63\x68\x75\x6e\x6b\x73\x5b\x69\x5d\x29\x0a\x20\x20\x7d\x0a\x7d\x0a\x0a\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x73\x74\x72\x65\x61\x6d\x57\x72\x69\x74\x65\x20\x28\x6d\x75\x78\x29\x20\x7b\x0a\x20\x20\x72\x65\x74\x75\x72\x6e\x20\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x28\x29\x20\x7b\x0a\x20\x20\x20\x20\x69\x66\x20\x28\x61\x72\x67\x75\x6d\x65\x6e\x74\x73\x2e\x6c\x65\x6e\x67\x74\x68\x20\x3e\x20\x30\x29\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x73\x6f\x63\x6b\x65\x74\x57\x72\x69\x74\x65\x28\x73\x6f\x63\x6b\x65\x74\x2c\x20\x6d\x75\x78\x2c\x20\x61\x72\x67\x75\x6d\x65\x6e\x74\x73\x5b\x30\x5d\x29\x0a\x20\x20\x20\x20\x7d\x0a\x20\x20\x7d\x0a\x7d\x0a\x0a\x66\x75\x6e\x63\x74\x69\x6f\x6e\x20\x73\x65\x74\x75\x70\x41\x73\x79\x6e\x63\x4d\x6f\x64\x65\x20\x28\x61\x73\x79\x6e\x63\x29\x20\x7b\x0a\x20\x20\x69\x66\x20\x28\x61\x73\x79\x6e\x63\x20\x3d\x3d\x3d\x20\x61\x73\x79\x6e\x63\x4d\x6f\x64\x65\x29\x20\x7b\x0a\x20\x20\x20\x20\x72\x65\x74\x75\x72\x6e\x0a\x20\x20\x7d\x0a\x20\x20\x61\x73\x79\x6e\x63\x4d\x6f\x64\x65\x20\x3d\x20\x61\x73\x79\x6e\x63\x0a\x0a\x20\x20\x69\x66\x20\x28\x21\x61\x73\x79\x6e\x63\x4d\x6f\x64\x65\x29\x20\x7b\x0a\x20\x20\x20\x20\x70\x72\x6f\x63\x65\x73\x73\x2e\x6f\x6e\x28\x27\x75\x6e\x63\x61\x75\x67\x68\x74\x45\x78\x63\x65\x70\x74\x69\x6f\x6e\x27\x2c\x20\x28\x65\x72\x72\x6f\x72\x29\x20\x3d\x3e\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x6c\x61\x73\x74\x43\x6f\x6e\x74\x65\x78\x74\x2e\x68\x61\x6e\x64\x6c\x65\x45\x72\x72\x6f\x72\x28\x65\x72\x72\x6f\x72\x29\x0a\x20\x20\x20\x20\x7d\x29\x0a\x0a\x20\x20\x20\x20\x2f\x2f\x20\x52\x65\x73\x74\x61\x72\x74\x20\x72\x65\x61\x64\x65\x72\x20\x62\x65\x66\x6f\x72\x65\x20\x65\x78\x69\x74\x2e\x0a\x20\x20\x20\x20\x70\x72\x6f\x63\x65\x73\x73\x2e\x6f\x6e\x28\x27\x62\x65\x66\x6f\x72\x65\x45\x78\x69\x74\x27\x2c\x20\x28\x63\x6f\x64\x65\x29\x20\x3d\x3e\x20\x7b\x0a\x20\x20\x20\x20\x20\x20\x73\x65\x72\x76\x65\x72\x2e\x72\x65\x66\x28\x29\x0a\x20\x20\x20\x20\x20\x20\x6c\x61\x73\x74\x43\x6f\x6e\x74\x65\x78\x74\x2e\x73\x65\x6e\x64\x52\x65\x73\x70\x6f\x6e\x73\x65\x28\x74\x72\x75\x65\x29\x0a\x20\x20\x20\x20\x7d\x29\x0a\x20\x20\x7d\x0a\x7d\x0a\x0a\x2f\x2f\x20\x53\x74\x61\x72\x74\x20\x6c\x69\x73\x74\x65\x6e\x69\x6e\x67\x2e\x0a\x6c\x65\x74\x20\x61\x64\x64\x72\x65\x73\x73\x20\x3d\x20\x6f\x73\x2e\x6e\x65\x74\x77\x6f\x72\x6b\x49\x6e\x74\x65\x72\x66\x61\x63\x65\x73\x28\x29\x5b\x27\x65\x74\x68\x30\x27\x5d\x5b\x30\x5d\x2e\x61\x64\x64\x72\x65\x73\x73\x0a\x73\x65\x72\x76\x65\x72\x2e\x6c\x69\x73\x74\x65\x6e\x28\x41\x50\x50\x5f\x50\x4f\x52\x54\x2c\x20\x61\x64\x64\x72\x65\x73\x73\x29\x0a\x73\x65\x72\x76\x65\x72\x2e\x6f\x6e\x28\x27\x6c\x69\x73\x74\x65\x6e\x69\x6e\x67\x27\x2c\x20\x28\x29\x20\x3d\x3e\x20\x7b\x0a\x20\x20\x6f\x72\x69\x67\x53\x74\x64\x6f\x75\x74\x57\x72\x69\x74\x65\x2e\x61\x70\x70\x6c\x79\x28\x70\x72\x6f\x63\x65\x73\x73\x2e\x73\x74\x64\x6f\x75\x74\x2c\x20\x5b\x61\x64\x64\x72\x65\x73\x73\x20\x2b\x20\x27\x3a\x27\x20\x2b\x20\x41\x50\x50\x5f\x50\x4f\x52\x54\x20\x2b\x20\x27\x5c\x6e\x27\x5d\x29\x0a\x7d\x29\x0a"

func wrappersNodeJsBytes() ([]byte, error) {
	return bindataRead(
		_wrappersNodeJs,
		"wrappers/node.js",
	)
}

func wrappersNodeJs() (*asset, error) {
	bytes, err := wrappersNodeJsBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "wrappers/node.js", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"wrappers/node.js": wrappersNodeJs,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}
var _bintree = &bintree{nil, map[string]*bintree{
	"wrappers": &bintree{nil, map[string]*bintree{
		"node.js": &bintree{wrappersNodeJs, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}

