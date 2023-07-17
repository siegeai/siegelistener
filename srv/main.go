package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/urfave/negroni"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type widget struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

var widgets = []widget{
	{
		ID:          "1",
		Title:       "Jeremy Bearimy",
		Description: "Some convoluted cyclical timeline situation",
	},
	{
		ID:          "2",
		Title:       "Gizmo",
		Description: "This is more of a gizmo than a widget, but we'll abuse the widget system to store it here.",
	},
}

func root(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome home!")
}

func createWidget(w http.ResponseWriter, r *http.Request) {
	var newWidget widget
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(w, "Kindly enter data with the widget title and description only in order to update")
	}

	json.Unmarshal(reqBody, &newWidget)
	widgets = append(widgets, newWidget)
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(newWidget)
}

func getWidget(w http.ResponseWriter, r *http.Request) {
	widgetID := mux.Vars(r)["id"]

	for _, singleWidget := range widgets {
		if singleWidget.ID == widgetID {
			json.NewEncoder(w).Encode(singleWidget)
		}
	}
}

func getWidgets(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(widgets)
}

func updateWidget(w http.ResponseWriter, r *http.Request) {
	widgetId := mux.Vars(r)["id"]
	var updatedWidget widget

	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(w, "Kindly enter data with the widget title and description only in order to update")
	}

	json.Unmarshal(reqBody, &updatedWidget)

	for i, singleWidget := range widgets {
		if singleWidget.ID == widgetId {
			singleWidget.Title = updatedWidget.Title
			singleWidget.Description = updatedWidget.Description
			widgets = append(widgets[:i], singleWidget)
			json.NewEncoder(w).Encode(singleWidget)
		}
	}
}

func deleteWidget(w http.ResponseWriter, r *http.Request) {
	widgetId := mux.Vars(r)["id"]

	for i, singleWidget := range widgets {
		if singleWidget.ID == widgetId {
			widgets = append(widgets[:i], widgets[i+1:]...)
			fmt.Fprintf(w, "The widget with ID %v has been deleted successfully", widgetId)
		}
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := negroni.NewResponseWriter(w)
		next.ServeHTTP(ww, r)
		log.Println(r.Method, r.RequestURI, r.Proto, "->", ww.Status(), http.StatusText(ww.Status()))
	})
}

func main() {
	host := flag.String("h", "", "the host to listen on")
	port := flag.String("p", "80", "the port to listen on")
	flag.Parse()

	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Println("Listening at", addr)

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", root)
	router.HandleFunc("/widget", createWidget).Methods("POST")
	router.HandleFunc("/widgets", getWidgets).Methods("GET")
	router.HandleFunc("/widgets/{id}", getWidget).Methods("GET")
	router.HandleFunc("/widgets/{id}", updateWidget).Methods("PATCH")
	router.HandleFunc("/widgets/{id}", deleteWidget).Methods("DELETE")
	router.Use(logMiddleware)
	log.Fatal(http.ListenAndServe(addr, router))
}
