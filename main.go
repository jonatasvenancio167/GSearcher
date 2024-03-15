package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	mongoURI       = "mongodb://admin:admin_password@localhost:27017"
	databaseName   = "searches"
	collectionName = "results"
	apiKey         = "AIzaSyB0mf1uldUV-I1l5Yj9JKiUt9yAknFWB6Y"
	cx             = "e4d2273544fcc4a81"
)

var (
	client     *mongo.Client
	ctx        context.Context
	customPath = "https://www.googleapis.com/customsearch/v1"
)

type SearchResponse struct {
	Items []struct {
		Title       string `json:"title"`
		Link        string `json:"link"`
		Description string `json:"snippet"`
	} `json:"items"`
}

type SearchItem struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"snippet"`
}

func initMongo() {
	clientOptions := options.Client().ApplyURI(mongoURI)
	var err error
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")
}

func getAllLists(w http.ResponseWriter, r *http.Request) {
	collection := client.Database(databaseName).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Erro ao encontrar documentos: %v", err), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var lists []interface{}
	for cursor.Next(ctx) {
		var list interface{}
		if err := cursor.Decode(&list); err != nil {
			http.Error(w, fmt.Sprintf("Erro ao decodificar documento: %v", err), http.StatusInternalServerError)
			return
		}
		lists = append(lists, list)
	}
	if err := cursor.Err(); err != nil {
		http.Error(w, fmt.Sprintf("Erro ao iterar sobre os documentos: %v", err), http.StatusInternalServerError)
		return
	}

	// Retorne os documentos como JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lists)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Query param 'query' is required", http.StatusBadRequest)
		return
	}

	url := fmt.Sprintf("%s?key=%s&cx=%s&q=%s", customPath, apiKey, cx, query)

	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error executing search: %v", err), http.StatusInternalServerError)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Erro ao ler o corpo da resposta: %v", err), http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()
	log.Println(resp)
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Erro: status da resposta não é 200 OK, é %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}

	var results SearchResponse
	err = json.NewDecoder(resp.Body).Decode(&results)

	if err != nil {
		http.Error(w, fmt.Sprintf("Erro ao analisar a resposta JSON: %v", err), http.StatusInternalServerError)
		return
	}

	// Salvar no MongoDB

	collection := client.Database(databaseName).Collection(collectionName)

	if collection == nil {
		log.Println("A coleção está nula")
		http.Error(w, "A coleção está nula", http.StatusInternalServerError)
		return
	}

	for _, item := range results.Items {

		insertResult, err := collection.InsertOne(ctx, item)
		if err != nil {
			log.Println("Erro ao salvar no MongoDB:", err)
		}

		if oid, ok := insertResult.InsertedID.(primitive.ObjectID); ok {
			fmt.Println("Documento inserido com sucesso. ID:", oid)
		} else {
			fmt.Println("Documento inserido com sucesso.")
		}
	}

	// Retornar os resultados como JSON

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func main() {
	initMongo()

	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/lists", getAllLists)

	fmt.Println("Server listening on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
