package test

import (
	"encoding/json"
	"fmt"
)

func PrettyPrintJson(obj interface{}) {
	j, _ := json.MarshalIndent(obj, "", "\t")
	fmt.Println(string(j))
}
