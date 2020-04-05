package main

import (
  "io/ioutil"
  "log"
  "net/http"
  "os"
)

var dataFilePath = "./cj.txt"

func newNoteHandler(w http.ResponseWriter, r *http.Request) {
  dataFile, err := os.OpenFile(dataFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
  if err != nil {
    log.Println("file open problem", err.Error())
  }
  defer dataFile.Close()

  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    log.Fatal(err)
  }

  s := "\n" + string(body) + "\n"
  _, err = dataFile.WriteString(s);
  if err != nil {
    log.Fatal(err)
  } else {
    log.Println("appended to file successfully")
  }
}

func main() {
  http.HandleFunc("/note/new", newNoteHandler)
  log.Fatal(http.ListenAndServe(":8080", nil))
}
