package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func PrettyPrintJson(obj interface{}) {
	j, _ := json.MarshalIndent(obj, "", "\t")
	fmt.Println(string(j))
}

type FakeResponseWriter struct {
	header http.Header
	id     string
}

func (w *FakeResponseWriter) Header() http.Header {
	return w.header
}

func (w *FakeResponseWriter) Write(b []byte) (int, error) {
	err := ioutil.WriteFile(fmt.Sprintf("../sandbox/%s.tar.gz", w.id), b, 0644)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (w *FakeResponseWriter) WriteHeader(statusCode int) {
	b, _ := json.MarshalIndent(w.header, "", "  ")
	_ = ioutil.WriteFile(fmt.Sprintf("../sandbox/%s.json", w.id), b, 0644)
	fmt.Printf("status code: %d", statusCode)
}

func NewFakeResponseWriter(id string) *FakeResponseWriter {
	return &FakeResponseWriter{
		header: make(http.Header),
		id:     id,
	}
}
