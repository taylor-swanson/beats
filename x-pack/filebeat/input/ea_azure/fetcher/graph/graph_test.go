package graph

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_Foo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	}))
	defer srv.Close()

	addr := srv.Listener.Addr()

	fmt.Println(addr.String())
}
