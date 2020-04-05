package main

import (
  "bufio"
  "io"
  "io/ioutil"
  "log"
  "net/http"
  "os"
  "os/exec"
)

var dataFilePath = "./cj.txt"

func newNoteHandler(w http.ResponseWriter, r *http.Request) {
  dataFile, err := os.OpenFile(dataFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
  if err != nil {
    log.Println("file open problem", err.Error())
  }

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
  dataFile.Close()

  // Go run our python script
  cmd := exec.Command("/usr/bin/python", "cp_txt.py")
  stdout, err := cmd.StdoutPipe()
  if err != nil {
    log.Fatal(err)
  }
  err = cmd.Start()
  if err != nil {
    log.Fatal(err)
  }
  go copyOutput(stdout)

}

func copyOutput(r io.Reader) {
  scanner := bufio.NewScanner(r)
  for scanner.Scan() {
    log.Println(scanner.Text())
  }
}

func main() {
  http.HandleFunc("/note/new", newNoteHandler)
  log.Fatal(http.ListenAndServe(":8080", nil))
}
