package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	//  "regexp"
	"encoding/json"

	"github.com/enescakir/emoji"
	"github.com/google/uuid"
	//"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgxpool"
)

var timeFormat = "2006 Jan _2 Mon 03:04:05.000 PM MST"
var streamFormat = "\n%v %v:\n%v\n"

var streamPath = "./cj.txt"
var devPath = "./cj_dev.txt"
var dataDir = "data/"

var streamDB = map[string][]Stream{}

var basePaths = [2]string{"/stream", "/dev"}
var filePath = map[string]string{
	"/stream": streamPath,
	"/dev":    devPath,
}

type PlayerAction string
type EntityType string
type StreamType string

const (
	Podcast        EntityType = "Podcast"
	PodcastEpisode EntityType = "PodcastEpisode"
	Thought        EntityType = "thought"
)

var emojiType = map[EntityType]string{
	Podcast:        "\U0001f399\ufe0f",           // :StudioMicrophone:
	PodcastEpisode: "\U0001f399\ufe0f\U0001f4c1", // :StudioMicrophone::FileFolder:
	Thought:        "\U0001f4ad",                 // :ThoughtBubble:
}

const (
	Play  PlayerAction = "play"
	Seek  PlayerAction = "seek"
	Pause PlayerAction = "pause"
	Stop  PlayerAction = "stop"
)

type DBEntity struct {
	Id   uuid.UUID
	name string
}

type Point struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}

type QuickNote struct {
	Content string
}

type StreamConfig struct {
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	EntityName string    `json:"entity_name"`
	Version    string    `json:"version"`
	UUID       uuid.UUID `json:"uuid"`
	Location   Point     `json:"location"`
	Production bool      `json:"prod"`
}

const (
	PublicStream StreamType = "stream"
	DevStream    StreamType = "dev"
)

type Stream struct {
	Time       string       `json:"time"`
	StreamType StreamType   `json:"stream_type"`
	Config     StreamConfig `json:"config"`
	Data       interface{}  `json:"data"`
}

type PathHandler struct {
	Path       string
	StreamType StreamType
	Conn       *pgxpool.Pool
}

type Entity struct {
	Id     int         `json:"-"`
	TypeId int         `json:"-"`
	Type   EntityType  `json:"type"`
	Data   interface{} `json:"data"`
}

type Player struct {
	//EntityId string
	Action   PlayerAction
	Position int
	Entity   Entity
}

func pythonCopy() {
	cmd := exec.Command("/usr/bin/python", "cp_txt.py")
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
}

func timeNow() string {
	t := time.Now()
	return t.Format(timeFormat)
}

func (ph *PathHandler) isStream() bool {
	return ph.Path == "/stream"
}

func (ph *PathHandler) appendToFile(fp string, s string) {
	file, err := os.OpenFile(fp, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	defer file.Close()

	if err != nil {
		log.Println("file open problem", err.Error())
	}

	// Write the new data to it
	_, err = file.WriteString(s)
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
	fp := filePath[ph.Path]
	ph.appendToFile(fp, s)

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
	s := fmt.Sprintf(streamFormat, es, timeNow(), string(body))
	fp := filePath[ph.Path]
	ph.appendToFile(fp, s)

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

	es := "\U0001f508"
	es += emojiType[p.Entity.Type]

	s := fmt.Sprintf(streamFormat, es, timeNow(), string(body))
	fp := filePath[ph.Path]
	ph.appendToFile(fp, s)

	pythonCopy()
}

func (ph *PathHandler) startActivityHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(body))
}

func (ph *PathHandler) stopActivityHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(body))
}

func (sp *StreamParser) parseQuickNote() (e error) {
	switch sp.StreamData.Config.Name {
	case "thoughts":
		// Add Entity to DB
		var data string
		e = json.Unmarshal(sp.Data, &data)
		if e != nil {
			log.Fatal(e)
		}
		sp.StreamData.Config.EntityName = data
		e = addNewEntityToDB(sp.StreamData.Config, sp.DBConn)
		if e != nil {
			log.Fatal(e)
		}

		rows, e := sp.DBConn.Query(context.Background(),
			"INSERT INTO quick_note (uuid, content) VALUES ($1, $2)",
			sp.StreamData.Config.UUID, data)
		if e != nil {
			log.Fatal(e)
		}

		defer rows.Close()

		if rows.Err() != nil {
			log.Fatal(rows.Err())
		}

	default:
		log.Fatal("gave horrible input:", sp.StreamData.Config.Name)
	}

	return e
}

func addNewEntityToDB(sc StreamConfig, conn *pgxpool.Pool) (e error) {
	locationString := fmt.Sprintf("ST_GeomFromText('POINT(%v %v)', 4326)", sc.Location.Lat, sc.Location.Long)
	log.Println("location is", locationString)
	rows, e := conn.Query(context.Background(),
		"INSERT INTO entity (id, name, prod) VALUES ($1, $2, $3)"+
			"ON CONFLICT (id) DO UPDATE SET name=$2, prod=$3",
		sc.UUID, sc.EntityName, sc.Production)
	if e != nil {
		log.Fatal(e)
	}

	defer rows.Close()

	if rows.Err() != nil {
		log.Fatal(rows.Err())
	}

	log.Println("inserted successfully into db entity")

	return e
}

func (sp *StreamParser) parseTest() (e error) {

	switch sp.StreamData.Config.Name {
	case "streamableLibrary":
		sp.StreamData.Config.EntityName = sp.StreamData.Config.Namespace +
			sp.StreamData.Config.Name
		e = addNewEntityToDB(sp.StreamData.Config, sp.DBConn)

		if e != nil {
			log.Fatal(e)
		}

		log.Println("data was:", string(sp.Data))

		// after getting data add it into the test table

		rows, e := sp.DBConn.Query(context.Background(),
			"INSERT INTO test (uuid, data) VALUES ($1, $2)",
			sp.StreamData.Config.UUID, string(sp.Data))
		if e != nil {
			log.Fatal(e)
		}

		defer rows.Close()

		if rows.Err() != nil {
			log.Fatal(rows.Err())
		}

		log.Println("inserted into test db successfully")

	default:
		log.Fatal("gave horrible input:", sp.StreamData.Config.Name)
	}

	return e
}

type PersistentEpisode struct {
	Id             uuid.UUID         `json:"id"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	PublishedDate  time.Time         `json:"published_date"`
	AudioURL       string            `json:"audio_url"`
	AudioLengthSec int               `json:"audio_length_sec"`
	ListenNotesID  string            `json:"listen_notes_episode_id"`
	Podcast        PersistentPodcast `json:"podcast"`
}

type PersistentPodcast struct {
	Id            uuid.UUID `json:"id"`
	ListenNotesID string    `json:"listen_notes_id"`
	Title         string    `json:"title"`
	Publisher     string    `json:"publisher"`
	Subscribed    bool      `json:"subscribed"`
	RSS           string    `json:"rss"`
	Image         string    `json:"image"`
	ImageURL      string    `json:"image_url"`
	Description   string    `json:"description"`
}

type PersistentBookmark struct {
	Id        uuid.UUID         `json:"id"`
	Timestamp int               `json:"timestamp"`
	CreatedAt time.Time         `json:"created_at"`
	Episode   PersistentEpisode `json:"episode"`
}

type StreamParser struct {
	StreamData Stream
	Data       json.RawMessage
	DBConn     *pgxpool.Pool
}

func secToHHMMSS(sec int) string {
	hours := sec / 3600
	minutes := (sec % 3600) / 60
	seconds := sec % 60

	if hours > 0 {
		return fmt.Sprintf("%v:%v:%v", hours, minutes, seconds)
	} else {
		return fmt.Sprintf("%v:%v", minutes, seconds)
	}

}

func (sp *StreamParser) parsePersistentBookmark() (e error) {
	// 1. Get our []byte into a reasonable format
	var pb PersistentBookmark
	e = json.Unmarshal(sp.Data, &pb)

	// 2. Add our new entity to the DB
	sp.StreamData.Config.EntityName = fmt.Sprintf("bookmark @ %v on %v",
		secToHHMMSS(pb.Timestamp), pb.Episode.Title)
	e = addNewEntityToDB(sp.StreamData.Config, sp.DBConn)

	log.Println("EPISODE ID", pb.Episode.Id)

	if e != nil {
		log.Fatal(e)
	}

	// 3. Insert into the DB
	rows, e := sp.DBConn.Query(
		context.Background(),
		"INSERT INTO podcast_episode_bookmark "+
			"(uuid, podcast_episode_id, timestamp)"+
			"VALUES ($1, $2, $3)"+
			"ON CONFLICT (uuid) DO UPDATE SET "+
			"podcast_episode_id=$2, timestamp=$3",
		sp.StreamData.Config.UUID, pb.Episode.Id, pb.Timestamp,
	)
	if e != nil {
		log.Fatal(e)
	}

	log.Printf("bookmark @ %v on %v",
		secToHHMMSS(pb.Timestamp), pb.Episode.Title)

	defer rows.Close()

	if rows.Err() != nil {
		log.Fatal(rows.Err())
	}

	return e
}

func (sp *StreamParser) parsePersistentEpisode() (e error) {
	// 1. Get our []byte into a reasonable format
	var ep PersistentEpisode
	e = json.Unmarshal(sp.Data, &ep)
	log.Printf("date %v uuid %v", ep.PublishedDate, sp.StreamData.Config.UUID)

	// 2. Add our new entity to the DB
	sp.StreamData.Config.EntityName = ep.Title
	e = addNewEntityToDB(sp.StreamData.Config, sp.DBConn)

	if e != nil {
		log.Fatal(e)
	}

	// 3. Insert into the DB
	rows, e := sp.DBConn.Query(
		context.Background(),
		"INSERT INTO podcast_episode "+
			"(uuid, name, description, published_date, audio_url,"+
			" audio_length_sec, podcast_id, listen_notes_id)"+
			"VALUES ($1, $2, $3, $4, $5, $6, $7, $8)"+
			"ON CONFLICT (uuid) DO UPDATE SET "+
			"name=$2, description=$3, published_date=$4, audio_url=$5,"+
			"audio_length_sec=$6, podcast_id=$7, listen_notes_id=$8",
		sp.StreamData.Config.UUID, ep.Title, ep.Description, ep.PublishedDate, ep.AudioURL,
		ep.AudioLengthSec, ep.Podcast.Id, ep.ListenNotesID,
	)
	if e != nil {
		log.Fatal(e)
	}

	defer rows.Close()

	if rows.Err() != nil {
		log.Fatal(rows.Err())
	}

	return e
}

func (sp *StreamParser) parsePersistentPodcast() (e error) {

	// 1. Get our []byte into a reasonable format
	var p PersistentPodcast
	e = json.Unmarshal(sp.Data, &p)
	log.Println(p)

	// 2. Add our new entity to the DB
	sp.StreamData.Config.EntityName = p.Title
	e = addNewEntityToDB(sp.StreamData.Config, sp.DBConn)

	if e != nil {
		log.Fatal(e)
	}

	// 3. Insert into the DB
	rows, e := sp.DBConn.Query(
		context.Background(),
		"INSERT INTO podcast "+
			"(uuid, name, publisher, description, rss_feed,"+
			" image, image_url, subscribed, listen_notes_id)"+
			"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"+
			"ON CONFLICT (uuid) DO UPDATE SET "+
			"name=$2, publisher=$3, description=$4, rss_feed=$5,"+
			"image=$6, image_url=$7, subscribed=$8, listen_notes_id=$9",
		sp.StreamData.Config.UUID, p.Title, p.Publisher, p.Description, p.RSS,
		p.Image, p.ImageURL, p.Subscribed, p.ListenNotesID,
	)
	if e != nil {
		log.Fatal(e)
	}

	defer rows.Close()

	if rows.Err() != nil {
		log.Fatal(rows.Err())
	}

	log.Println("inserted into test db successfully")

	return e
}

func (sp *StreamParser) parsePodcast() (e error) {

	switch sp.StreamData.Config.Name {
	case "PersistentPodcast":
		e = sp.parsePersistentPodcast()
	case "PersistentEpisode":
		e = sp.parsePersistentEpisode()
	case "PersistentBookmark":
		e = sp.parsePersistentBookmark()
	}

	return e
}

func (sp *StreamParser) parseStreamInput() (e error) {
	e = nil

	// really sill case statement
	switch sp.StreamData.Config.Namespace {
	case "cj/notes":
		e = sp.parseQuickNote()
	case "cj/test":
		e = sp.parseTest()
	case "cj/podcast":
		e = sp.parsePodcast()
	default:
		log.Fatalf("couldnt parse non existent namespace %v on stream %v",
			sp.StreamData.Config.Namespace, sp.StreamData.StreamType)
	}

	return e
}

var mutex = sync.RWMutex{}

func (ph *PathHandler) streamInputHandler(w http.ResponseWriter, r *http.Request) {
	// Lock becasue we can have concurrent map access
	mutex.Lock()
	defer mutex.Unlock()

	log.Println("hit stream handler")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Make sure we are going to interpret data as another []byte for later decoding
	var data json.RawMessage
	stream := Stream{
		Data: &data,
	}
	err = json.Unmarshal(body, &stream)
	if err != nil {
		log.Fatal(err)
	}

	// Setting some server side params for remarshalling
	stream.Time = timeNow()

	// Set the streamtype for bookeeping
	stream.StreamType = ph.StreamType
	if stream.StreamType == DevStream {
		stream.Config.Production = false
	} else {
		stream.Config.Production = true
	}

	bytestream, err := json.Marshal(stream)
	if err != nil {
		log.Fatal(err)
	}

	namespace := stream.Config.Namespace + "/" + stream.Config.Name
	streamDB[namespace] = append(streamDB[namespace], stream)

	// Make dir if it doesnt exist
	dirpath := dataDir + stream.Config.Namespace
	os.MkdirAll(dirpath, 0755)

	s := fmt.Sprintf("%v\n", string(bytestream))
	fp := fmt.Sprintf("%v/%v.txt", dirpath, stream.Config.Name)
	ph.appendToFile(fp, s)

	// Go do DB stuff
	sp := StreamParser{
		StreamData: stream,
		Data:       data,
		DBConn:     ph.Conn,
	}
	err = sp.parseStreamInput()
	if err != nil {
		log.Fatal(err)
	}

}

func (ph *PathHandler) init(path string) {
	ph.Path = path
	if path == basePaths[0] {
		ph.StreamType = PublicStream
	} else {
		ph.StreamType = DevStream
	}
}

func buildStreamDB() {
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			namespace := path[len(dataDir):]
			namespace = strings.TrimSuffix(namespace, filepath.Ext(namespace))
			//var tmpStream Stream

			// Handle opening the file
			file, err := os.Open(path)
			defer file.Close()

			if err != nil {
				log.Println("file open problem", err.Error())
			}

			// Read lines from the file and insert them into the DB
			reader := bufio.NewReader(file)
			var line string
			var stream Stream
			for {
				line, err = reader.ReadString('\n')
				if err != nil && err != io.EOF {
					break
				}

				// Only process real data
				if line != "\n" && line != "" {
					err = json.Unmarshal([]byte(line), &stream)
					if err != nil {
						log.Fatal(err)
					}
					if _, ok := streamDB[namespace]; ok {
						streamDB[namespace] = append(streamDB[namespace], stream)
					} else {
						initArr := []Stream{stream}
						streamDB[namespace] = initArr
					}
				}

				if err != nil {
					break
				}
			}

		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	args := os.Args[1:]
	port := ":8080"

	buildStreamDB()

	if len(args) > 0 && args[0] == "debug" {
		port = ":10000"
	}

	for i := 0; i < 2; i++ {
		ph := &PathHandler{}
		ph.init(basePaths[i])
		conn, err := pgxpool.Connect(context.Background(), "postgres://cj:20conroe09@localhost:5432/cj_test2")
		ph.Conn = conn
		if err != nil {
			log.Fatal(err)
		}

		http.HandleFunc(ph.Path+"/stream", ph.streamInputHandler)
		http.HandleFunc(ph.Path+"/new/thought", ph.newThoughtHandler)
		http.HandleFunc(ph.Path+"/new/entity", ph.newEntityHandler)
		http.HandleFunc(ph.Path+"/action/player", ph.playerActionHandler)
		http.HandleFunc(ph.Path+"/start/activity", ph.startActivityHandler)
		http.HandleFunc(ph.Path+"/stop/activity", ph.stopActivityHandler)
	}

	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)

	log.Fatal(http.ListenAndServe(port, nil))
}
