//go:build js && wasm

package eclass

import (
	"fmt"
	"syscall/js"
)

// Workers에는 파일시스템이 없다. 호스트가 심어준 두 함수로 대신한다.
//
//	globalThis.eclassStoreGet(name) -> Promise<string|null>
//	globalThis.eclassStoreSet(name, value|null) -> Promise<void>
//
// worker/src/index.js 가 credentials는 시크릿에서, session은 KV에서 꺼내 준다.
func storeRead(name string) ([]byte, error) {
	v, err := awaitJS(js.Global().Call("eclassStoreGet", name))
	if err != nil {
		return nil, err
	}
	if v.IsNull() || v.IsUndefined() {
		return nil, fmt.Errorf("%s: 저장된 값 없음", name)
	}
	return []byte(v.String()), nil
}

func storeWrite(name string, d []byte) error {
	_, err := awaitJS(js.Global().Call("eclassStoreSet", name, string(d)))
	return err
}

func storeRemove(name string) error {
	_, err := awaitJS(js.Global().Call("eclassStoreSet", name, js.Null()))
	return err
}
