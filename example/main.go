package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("listening on http://localhost:3002")
	if err := http.ListenAndServe(":3002", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})); err != nil {
		fmt.Println(err)
	}
}
