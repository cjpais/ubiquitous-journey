package main

import (
  "io/ioutil"
  "log"
  "fmt"
  "net/http"
  "os"
  "os/exec"
  "time"
  "encoding/json"

  "github.com/enescakir/emoji"
)
var timeFormat = "2006 Jan _2 Mon 03:04:05.000 PM MST"
var streamFormat = "\n%v %v:\n%v\n"

var streamPath = "./cj.txt"
var devPath = "./cj_dev.txt"

var basePaths = [2]string{"/stream", "/dev"}
var filePath = map[string]string{
  "/stream": streamPath,
  "/dev": devPath,
}

type PlayerAction string
type EntityType string

const (
  Podcast EntityType = "Podcast"
  PodcastEpisode EntityType = "PodcastEpisode"
  Thought EntityType = "thought"
)

var emojiType = map[EntityType]string{
  Podcast: "\U0001f399\ufe0f",                    // :StudioMicrophone:
  PodcastEpisode: "\U0001f399\ufe0f\U0001f4c1",   // :StudioMicrophone::FileFolder:
  Thought: "\U0001f4ad",                          // :ThoughtBubble:
}

const (
  Play PlayerAction = "play"
  Seek PlayerAction = "seek"
  Pause PlayerAction = "pause"
  Stop PlayerAction = "stop"
)

type PathHandler struct {
  Path string
}

type Entity struct {
  Id      int           `json:"-"`
  TypeId  int           `json:"-"`
  Type    EntityType    `json:"type"`
  Data    interface{}   `json:"data"`
}

type Player struct {
  //EntityId string
  Action PlayerAction
  Position int
  Entity Entity
}

func pythonCopy() {
  cmd := exec.Command("/usr/bin/python", "cp_txt.py")
  err := cmd.Start()
  if err != nil {
    log.Fatal(err)
  }
}

func timeNow() (string) {
  t := time.Now()
  return t.Format(timeFormat)
}


func (ph *PathHandler) isStream() (bool) {
  return ph.Path == "/stream"
}

func (ph *PathHandler) appendToFile(s string) {
  // Open file as append 
  fp := filePath[ph.Path]
  file, err := os.OpenFile(fp, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
  defer file.Close()

  if err != nil {
    log.Println("file open problem", err.Error())
  }

  // Write the new data to it
  _, err = file.WriteString(s);
  if err != nil {
    log.Fatal(err)
  } else {
    log.Println("appended to file successfully on", ph.Path)
  }
}

func (ph *PathHandler) newThoughtHandler(w http.ResponseWriter, r *http.Request) {
  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    log.Fatal(err)
  }

  s := fmt.Sprintf(streamFormat, emoji.ThoughtBalloon, timeNow(), string(body))
  ph.appendToFile(s)

  pythonCopy()
}

func (ph *PathHandler) newEntityHandler(w http.ResponseWriter, r *http.Request) {
  var e Entity

  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    log.Fatal(err)
  }
  err = json.Unmarshal(body, &e)

  log.Println(string(body))

  es := emojiType[e.Type]
  log.Println(es)

  // TODO need to assign the entity an ID and make sure there are no duplicate entities. probably store all in memory

  // TODO make this more generic
  s := fmt.Sprintf(streamFormat, es, timeNow(), string(body))
  ph.appendToFile(s)

  // TODO call python script to get into web format 
  pythonCopy()
}

func (ph *PathHandler) playerActionHandler(w http.ResponseWriter, r *http.Request) {
  var p Player

  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    log.Fatal(err)
  }
  err = json.Unmarshal(body, &p)
  log.Println(string(body))

  // TODO need to get the entity type
  es := "\U0001f508"
  es += emojiType[p.Entity.Type]

  s := fmt.Sprintf(streamFormat, es, timeNow(), string(body))
  ph.appendToFile(s)

  // TODO call python script to get into web format 
  pythonCopy()
}

func (ph *PathHandler) startActivityHandler(w http.ResponseWriter, r *http.Request) {
  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    log.Fatal(err)
  }
  log.Println(string(body))

  // TODO need to get the entity type

  // TODO make this more generic
  //s := fmt.Sprintf(streamFormat, emoji.ThoughtBalloon, timeNow(), string(body))
  //ph.appendToFile(s)

  // TODO call python script to get into web format 
}

func (ph *PathHandler) stopActivityHandler(w http.ResponseWriter, r *http.Request) {
  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    log.Fatal(err)
  }
  log.Println(string(body))

  // TODO need to get the entity type

  // TODO make this more generic
  //s := fmt.Sprintf("\n%v @ %v:\n %v\n", emoji.ThoughtBalloon, timeNow(), string(body))
  //ph.appendToFile(s)

  // TODO call python script to get into web format 
}

func main() {
  for i := 0; i < 2; i++ {
    ph := &PathHandler{Path: basePaths[i]}

    http.HandleFunc(ph.Path + "/new/thought", ph.newThoughtHandler)
    http.HandleFunc(ph.Path + "/new/entity", ph.newEntityHandler)
    http.HandleFunc(ph.Path + "/action/player", ph.playerActionHandler)
    http.HandleFunc(ph.Path + "/start/activity", ph.startActivityHandler)
    http.HandleFunc(ph.Path + "/stop/activity", ph.stopActivityHandler)
  }

  fs := http.FileServer(http.Dir("."))
  http.Handle("/", fs)

  log.Fatal(http.ListenAndServe(":8080", nil))
}
