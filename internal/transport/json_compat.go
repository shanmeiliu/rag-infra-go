package transport

import (
	"encoding/json"
	"net/http"
)

func jsonNewDecoder(r *http.Request) *json.Decoder {
	return json.NewDecoder(r.Body)
}
