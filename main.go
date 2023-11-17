package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"

	"gopkg.in/mgo.v2/bson"

	//"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var rnd *renderer.Render
var collection *mongo.Collection

const (
	hostName       string = "mongodb://localhost:8081"
	dbName         string = "demo"
	collectionName string = "todo"
	port           string = ":8081"
)

type (
	todoModel struct {
		ID        primitive.ObjectID `bson:"_id,omitempty"`
		Title     string             `bson:"title"`
		Completed bool               `bson:"completed"`
		CreateAt  time.Time          `bson:"createAt"`
	}
	todo struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Completed bool      `json:"completed"`
		CreateAt  time.Time `json:"create_at"`
	}
)

func init() {

	rnd = renderer.New()
	clientOptions := options.Client().ApplyURI(hostName)

	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(context.TODO(), nil)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")

	collection = client.Database(dbName).Collection(collectionName)
	fmt.Println("Collection Instance is Created!")

}
func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	checkErr(err)
}

func fetchTodo(w http.ResponseWriter, r *http.Request) {
	todos := []todoModel{}
	cursor, err := collection.Find(context.Background(), bson.M{})

	if err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "fialed to fetch todo",
			"error":   err,
		})
		return
	}
	if err = cursor.All(context.TODO(), &todos); err != nil {
		log.Fatal(err)
	}

	todoList := []todo{}

	for _, t := range todos {
		todoList = append(todoList, todo{
			ID:        t.ID.Hex(),
			Title:     t.Title,
			Completed: t.Completed,
			CreateAt:  t.CreateAt,
		})
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"data": todoList,
	})
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	fmt.Print("createTODO Called")
	var t todo

	err := json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}
	// var bs []byte
	// r.Body.Read(bs)
	// fmt.Println(string(bs))
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "the title is required",
		})
		return
	}

	tm := todoModel{
		ID:        primitive.NewObjectID(),
		Title:     t.Title,
		Completed: false,
		CreateAt:  time.Now(),
	}

	insertResult, err := collection.InsertOne(context.Background(), tm)

	if err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to save todo",
			"error":   err,
		})
		return
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "todo created successfully",
		"todo_id": insertResult.InsertedID,
	})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "the id is invalid",
		})
		return
	}

	idPrimitive, _ := primitive.ObjectIDFromHex(id)

	res, err := collection.DeleteOne(context.Background(), bson.M{"_id": idPrimitive})
	fmt.Print(res)
	if err != nil {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Failed to delete the id",
			"error":   err,
		})
		return
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "todo  deleted successfully",
	})
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Invalid id",
		})
		return
	}

	t := todo{}
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "the title is required",
		})
		return
	}
	idPrimitive, _ := primitive.ObjectIDFromHex(id)

	_, err := collection.UpdateOne(context.TODO(), bson.M{"_id": idPrimitive}, bson.M{"$set": bson.M{"title": t.Title, "Completed": t.Completed}})
	if err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "fialed to update",
			"error":   err,
		})
		return
	}

}

func main() {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Mount("/todo", todoHandler())

	srv := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("Listening on port", port)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Listen:%s\n", err)

		}
	}()
	<-stopChan
	log.Printf("Shutting down server........")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)
	defer cancel()
	log.Println("server gracefully Stopped!")

}

func todoHandler() http.Handler {
	rg := chi.NewRouter()
	rg.Group(func(r chi.Router) {
		r.Get("/", fetchTodo)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg

}
func checkErr(err error) {
	if err != nil {

		log.Fatal(err)
	}
}
