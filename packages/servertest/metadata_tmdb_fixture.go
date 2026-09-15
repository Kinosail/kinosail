package servertest

import (
	"net/http"
	"strings"
)

func serveMetadataTMDB(writer http.ResponseWriter, request *http.Request) {
	if strings.HasPrefix(request.URL.Path, "/3/") && request.Header.Get("Authorization") != "Bearer test-token" {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	switch request.URL.Path {
	case "/3/search/movie":
		if request.URL.Query().Get("query") == "Primer" && request.URL.Query().Get("primary_release_year") == "2004" {
			_, _ = writer.Write([]byte(`{"results":[{"id":14337}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"results":[]}`))
	case "/3/movie/14337":
		_, _ = writer.Write([]byte(`{"title":"Primer","release_date":"2004-10-08","overview":"Two engineers discover time travel.","poster_path":"/primer.jpg","genres":[{"name":"Science Fiction"}],"credits":{"cast":[{"name":"Shane Carruth","character":"Aaron","order":0,"profile_path":"/shane.jpg"}],"crew":[{"name":"Shane Carruth","job":"Director","department":"Directing"}]}}`))
	case "/images/primer.jpg":
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write([]byte("poster"))
	case "/images/shane.jpg":
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write([]byte("headshot"))
	default:
		http.NotFound(writer, request)
	}
}
