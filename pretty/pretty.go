package pretty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tidwall/pretty"
)

// PrettyWriter uses json marshal to pretty output an interface object
func PrettyWriter(object interface{}, writer io.Writer) {
	objectString, err := json.MarshalIndent(object, "", "  ")
	check(err)
	_, err = writer.Write(objectString)
	check(err)
}

// PrefixPrettyWriter uses json marshal to pretty output an interface object
func PrefixPrettyWriter(writer io.Writer, prefix string, object interface{}) {
	objectString, err := json.Marshal(object)
	check(err)

	if prefix != "" {
		prefix += ": "
	}

	_, err = fmt.Fprintf(writer, "%s%s\n", prefix, pretty.Pretty(objectString))
	check(err)
}

// PrefixPretty uses json marshal to pretty print an interface object
func PrefixPretty(prefix string, object interface{}) {
	PrefixPrettyWriter(os.Stdout, prefix, object)
}

// PrettyPrint uses json marshal to pretty print an interface object
func PrettyPrint(object interface{}) {
	PrefixPretty("", object)
}

func PrettyString(object interface{}) string {
	var buf bytes.Buffer
	PrefixPrettyWriter(&buf, "", object)
	return buf.String()
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
