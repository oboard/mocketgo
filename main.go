package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"

	"github.com/bytecodealliance/wasmtime-go/v25"
)

func main() {
	// 解析命令行参数
	filename := flag.String("PATH", "main.wasm", "wasm file")
	flag.Parse()

	// fmt.Println("PATH:", *filename)

	// Almost all operations in wasmtime require a contextual `store`
	// argument to share, so create that first
	linker := wasmtime.NewLinker(wasmtime.NewEngine())
	store := wasmtime.NewStore(wasmtime.NewEngine())

	// Compiling modules requires WebAssembly binary input, but the wasmtime
	// package also supports converting the WebAssembly text format to the
	// binary format.
	// wasm, err := wasmtime.Wat2Wasm(`
	//   (module
	//     (import "" "hello" (func $hello))
	//     (func (export "run")
	//       (call $hello))
	//   )
	// `)

	// Once we have our binary `wasm` we can compile that into a `*Module`
	// which represents compiled JIT code.
	// module, err := wasmtime.NewModule(store.Engine, wasm)
	// check(err)
	module, err := wasmtime.NewModuleFromFile(store.Engine, *filename)
	check(err)

	// Our `hello.wat` file imports one item, so we create that function
	// here.
	// item := wasmtime.WrapFunc(store, func() {
	// 	fmt.Println("Hello from Go!")
	// })

	// `(func (param i32))
	linker.DefineFunc(store, "spectest", "print_char",
		func(arg int32) {
			fmt.Printf("%c", arg)
		})

	var buffer []byte

	linker.DefineFunc(store, "__h", "h_sd",
		func(arg int32) {
			if arg != 0 {
				buffer = append(buffer, byte(arg))
			}
		})

	linker.DefineFunc(store, "__h", "h_se",
		func() {
			// 反序列化 JSON 字符串
			var data []interface{}
			err := json.Unmarshal(buffer, &data)
			check(err)

			// 打印 JSON 数据
			fmt.Println(data)
			args := data[1:]
			switch data[0].(string) {
			case "http.listen":
				http.ListenAndServe(":"+args[0].(string), nil)
			case "http.handle":
				http.HandleFunc(args[0].(string), func(w http.ResponseWriter, r *http.Request) {
					// 处理请求
					w.Write([]byte("Hello, World!"))
				})
			}
			// 清空缓冲区
			buffer = buffer[:0]
		})

	// Next up we instantiate a module which is where we link in all our
	// imports. We've got one import so we pass that in here.
	instance, err := linker.Instantiate(store, module)
	// wasmtime.NewInstance(store, module, []wasmtime.AsExtern{item, item, item})
	check(err)

	// After we've instantiated we can lookup our `run` function and call
	// it.
	run := instance.GetFunc(store, "_start")
	if run == nil {
		panic("not a function")
	}
	_, err = run.Call(store)
	check(err)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
