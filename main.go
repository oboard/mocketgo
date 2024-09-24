package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/bytecodealliance/wasmtime-go/v25"
)

func main() {
	port := 4000
	// 解析命令行参数
	filename := flag.String("PATH", "main.wasm", "wasm file")
	flag.Parse()
	var sendMessage = func(method string, json interface{}) {}
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

	var responses []http.ResponseWriter

	linker.DefineFunc(store, "__h", "h_se",
		func() {
			// 反序列化 JSON 字符串
			var data []interface{}
			err := json.Unmarshal(buffer, &data)
			check(err)

			// 打印 JSON 数据
			args := data[1:]
			switch data[0].(string) {
			case "http.listen":
				port = int(args[0].(float64))
			case "http.handle":
				http.HandleFunc(args[0].([]interface{})[1].(string), func(w http.ResponseWriter, r *http.Request) {
					responses = append(responses, w)
					if args[0].([]interface{})[0].(string) == r.Method {
						sendMessage("http.request", []interface{}{
							map[string]interface{}{
								"method": r.Method,
								"url":    r.URL.String(),
								"path":   r.URL.Path,
							},
							map[string]interface{}{
								"id": len(responses) - 1,
							},
						})
					}
				})
			case "http.end":
				args = args[0].([]interface{})
				id := int(args[0].(float64))
				response := responses[id]
				if response != nil {
					statusCode := int(args[1].(float64))
					headers := args[2].(map[string]interface{})
					body := args[3]

					// 写入响应头
					for key, value := range headers {
						response.Header().Set(key, value.(string))
					}

					// 写入响应状态码
					response.WriteHeader(statusCode)

					// 写入响应体
					if body != nil {
						// 判断是否是字符串类型
						if str, ok := body.(string); ok {
							_, err := response.Write([]byte(str))
							check(err)
						} else {
							// 如果是对象类型，序列化后写入
							if body, ok := body.(map[string]interface{}); ok {
								if body["_T"] == "file" {
									// 处理文件类型
									if body["path"] != nil {
										// 处理文件类型
										filePath := body["path"].(string)
										// 读取文件内容
										fileContent, err := os.ReadFile(filePath)
										if err != nil {
											// 处理文件读取错误
											fmt.Println("Error reading file:", err)
										} else {
											// 写入文件内容到响应
											_, err = response.Write(fileContent)
											check(err)
										}
									} else {
										// 处理其他情况
										fmt.Println("Invalid file object:", body)
									}
								}
							}

							bytes, err := json.Marshal(body)
							check(err)
							_, err = response.Write(bytes)
							check(err)
						}
					}

					// 关闭响应
					responses[id] = nil
				}
			}

			// 清空缓冲区
			buffer = buffer[:0]
		})

	// Next up we instantiate a module which is where we link in all our
	// imports. We've got one import so we pass that in here.
	instance, err := linker.Instantiate(store, module)
	// wasmtime.NewInstance(store, module, []wasmtime.AsExtern{item, item, item})
	check(err)

	h_rd := instance.GetExport(store, "h_rd").Func()
	h_re := instance.GetExport(store, "h_re").Func()

	sendMessage = func(method string, body interface{}) {
		bytes, err := json.Marshal([]interface{}{method, body})
		check(err)
		for i := 0; i < len(bytes); i++ {
			_, err := h_rd.Call(store, int32(bytes[i]))
			check(err)
		}
		_, err = h_re.Call(store)
		check(err)
	}

	// After we've instantiated we can lookup our `run` function and call
	// it.
	run := instance.GetFunc(store, "_start")
	if run == nil {
		panic("not a function")
	}
	_, err = run.Call(store)
	check(err)

	http.ListenAndServe(":"+strconv.Itoa(port), nil)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
