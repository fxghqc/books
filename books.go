package main

import (
	// "fmt"
	// "strconv"
	"github.com/StephanDollberg/go-json-rest-middleware-jwt"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"time"
)

type Book struct {
	Id        int64      `json:"id"`
	Name      string     `sql:"size:1024" json:"name"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"-"`
}

type Impl struct {
	DB *gorm.DB
}

func (i *Impl) InitDB() {
	var err error
	i.DB, err = gorm.Open("postgres", "postgresql://postgres:123456Pg@localhost:5432/postgres?sslmode=disable")
	if err != nil {
		log.Fatalf("Got error when connect database, the error is '%v'", err)
	}
	i.DB.LogMode(true)
}

func (i *Impl) InitSchema() {
	i.DB.AutoMigrate(&Book{})
}

func (i *Impl) GetAllBooks(w rest.ResponseWriter, r *rest.Request) {
	books := []Book{}
	i.DB.Find(&books)
	w.WriteJson(&books)
}

func (i *Impl) GetBook(w rest.ResponseWriter, r *rest.Request) {
	id := r.PathParam("id")
	book := Book{}

	if i.DB.First(&book, id).Error != nil {
		rest.NotFound(w, r)
		return
	}

	w.WriteJson(book)
}

func (i *Impl) PostBook(w rest.ResponseWriter, r *rest.Request) {
	book := Book{}
	if err := r.DecodeJsonPayload(&book); err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := i.DB.Save(&book).Error; err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(&book)
}

func (i *Impl) PutBook(w rest.ResponseWriter, r *rest.Request) {
	id := r.PathParam("id")
	book := Book{}
	if i.DB.First(&book, id).Error != nil {
		rest.NotFound(w, r)
		return
	}

	updated := Book{}
	if err := r.DecodeJsonPayload(&updated); err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	book.Name = updated.Name
	if err := i.DB.Save(&book).Error; err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteJson(&book)
}

func (i *Impl) DeleteBook(w rest.ResponseWriter, r *rest.Request) {
	id := r.PathParam("id")
	book := Book{}
	if i.DB.First(&book, id).Error != nil {
		rest.NotFound(w, r)
		return
	}
	if err := i.DB.Delete(&book).Error; err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handle_auth(w rest.ResponseWriter, r *rest.Request) {
	w.WriteJson(map[string]string{"authed": r.Env["REMOTE_USER"].(string)})
}

func main() {
	jwt_middleware := &jwt.JWTMiddleware{
		Key:        []byte("secret key"),
		Realm:      "jwt auth",
		Timeout:    time.Hour * 12,
		MaxRefresh: time.Hour * 24,
		Authenticator: func(userId string, password string) bool {
			return userId == "admin" && password == "admin"
		}}

	i := Impl{}
	i.InitDB()
	i.InitSchema()

	api := rest.NewApi()

	statusMw := &rest.StatusMiddleware{}
	api.Use(statusMw)

	api.Use(rest.DefaultDevStack...)

	api.Use(&rest.CorsMiddleware{
		RejectNonCorsRequests: false,
		OriginValidator: func(origin string, request *rest.Request) bool {
			// return origin == "http://my.other.host"
			return true
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{
			"Accept", "Content-Type", "X-Custom-Header", "Origin", "Authorization"},
		AccessControlAllowCredentials: true,
		AccessControlMaxAge:           3600,
	})

	api.Use(&rest.IfMiddleware{
		Condition: func(request *rest.Request) bool {
			return request.URL.Path != "/login"
		},
		IfTrue: jwt_middleware,
	})

	router, err := rest.MakeRouter(
		rest.Post("/login", jwt_middleware.LoginHandler),
		rest.Get("/auth_test", handle_auth),
		rest.Get("/refresh_token", jwt_middleware.RefreshHandler),
		rest.Get("/books", i.GetAllBooks),
		rest.Post("/books", i.PostBook),
		rest.Get("/books/:id", i.GetBook),
		rest.Put("/books/:id", i.PutBook),
		rest.Delete("/books/:id", i.DeleteBook),
		rest.Get("/.status", func(w rest.ResponseWriter, r *rest.Request) {
			w.WriteJson(statusMw.GetStatus())
		}),
	)

	if err != nil {
		log.Fatal(err)
	}

	api.SetApp(router)
	log.Fatal(http.ListenAndServe(":18080", api.MakeHandler()))
}
