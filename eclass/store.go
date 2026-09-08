//go:build !js

package eclass

import (
	"os"
	"path/filepath"
)

// 세션과 credentials는 홈 디렉터리에 둔다. Workers(wasm) 빌드는 store_js.go 를 쓴다.
func storePath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".eclass-"+name+".json")
}

func storeRead(name string) ([]byte, error)  { return os.ReadFile(storePath(name)) }
func storeWrite(name string, d []byte) error { return os.WriteFile(storePath(name), d, 0600) }
func storeRemove(name string) error          { return os.Remove(storePath(name)) }
