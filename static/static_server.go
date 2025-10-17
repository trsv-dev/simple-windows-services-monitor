package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	var (
		port = flag.String("port", "3000", "Порт для сервера статики")
		dir  = flag.String("dir", "./", "Папка со статическими файлами")
	)
	flag.Parse()

	// абсолютный путь к статическим файлам
	staticDir, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("Ошибка получения абсолютного пути: %v", err)
	}
	if info, err := os.Stat(staticDir); err != nil || !info.IsDir() {
		log.Fatalf("Каталог недоступен или не существует: %v", err)
	}

	// файловый сервер
	fs := http.FileServer(http.Dir(staticDir))

	// создаем новый (не дефолтовый) роутер
	mux := http.NewServeMux()

	// Обработчик для корня - отдает index.html
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})

	address := ":" + *port
	fmt.Printf("Сервер статики запущен на http://localhost%s\n", address)
	fmt.Printf("Раздается папка: %s\n", staticDir)

	log.Fatal(http.ListenAndServe(address, mux))
}
