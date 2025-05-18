package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/skip2/go-qrcode"
)

func main() {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Panicf("error: %v", err)
	}

	var localIp string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				localIp = ipnet.IP.String()
				break
			}
		}
	}
	if localIp == "" {
		log.Panicf("could not get local ip")
	}

	port := "8080"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	localUrl := &url.URL{
		Host:   net.JoinHostPort(localIp, port),
		Scheme: "http",
	}
	if _, err := url.Parse(localUrl.String()); err != nil {
		log.Panicf("failed to create url for local server: %v", err)
	}

	qr, err := qrcode.New(localUrl.String(), qrcode.Medium)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("listening and serving at %v\n%v", localUrl, qr.ToString(false))

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.Handle("/", http.StripPrefix("/", fs))
	http.Handle("/upload", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			log.Printf("got method %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		r.ParseMultipartForm(10 << 20)
		photo, handler, err := r.FormFile("photo")
		if err != nil {
			log.Printf("error retrieving the file: %v", err)
			http.Error(w, "error retrieving the file", http.StatusBadRequest)
			return
		}
		defer photo.Close()

		// XXX: taking a file name and putting into the filepath is insecure.
		uploadPath := fmt.Sprintf("./uploads/%s", handler.Filename)
		f, err := os.OpenFile(uploadPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("failed to open file to upload: %v", err)
			http.Error(w, "failed to upload file", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if _, err := io.Copy(f, photo); err != nil {
			log.Printf("failed to write upload to file: %v", err)
			http.Error(w, "failed to upload file", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		log.Printf("successfully uploaded %v", handler.Filename)
	}))

	log.Fatal(http.ListenAndServe(localUrl.Host, nil))
}
