package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/StephanDollberg/go-json-rest-middleware-jwt"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
)

// Book ...
type Book struct {
	ID          int64      `json:"id"`
	Name        string     `sql:"size:1024" json:"name"`
	Author      string     `sql:"size:512" json:"author"`
	Translator  string     `sql:"size:512" json:"translator"`
	Pages       int64      `json:"pages"`
	Publisher   string     `sql:"size:256" json:"publisher"`
	Language    string     `sql:"size:128" json:"language"`
	Description string     `sql:"size:" json:"description"`
	Quantity    int        `json:"quantity"`
	Owner       User       `json:"owner"`
	OwnerID     int64      `json:"ownerID"`
	Borrowers   []User     `gorm:"many2many:book_borrowers" sql:"size:1024" json:"borrowers"`
	PublishedAt *time.Time `json:"publishedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"-"`
	// Review    int64      `json:"review"`
	// Rank      string     `sql:"size:1024" json:"rank"`
}

// User ...
type User struct {
	ID        int64      `json:"id"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"-"`
	Name      string     `json:"name"`
	Password  string     `json:"-"`
	Email     string     `json:"email"`
}

// BorrowRecord ...
type BorrowRecord struct {
	ID        int64      `json:"id"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"-"`
	StartAt   *time.Time `json:"startAt"`
	EndAt     *time.Time `json:"endAt"`
	Book      Book       `json:"book"`
	BookID    int64      `json:"bookID"`
	User      User       `json:"user"`
	UserID    int64      `json:"userID"`
	Status    string     `sql:"size:128" json:"status"`
}

// Borrowing ...
type Borrowing struct {
	User *User `json:"user"`
	Book *Book `json:"book"`
}

// Impl ...
type Impl struct {
	DB *gorm.DB
}

// InitDB ...
func (i *Impl) InitDB() {
	var err error
	i.DB, err = gorm.Open("postgres", "postgresql://postgres:123456Pg@localhost:5442/postgres?sslmode=disable")
	if err != nil {
		log.Fatalf("Got error when connect database, the error is '%v'", err)
	}
	i.DB.LogMode(true)
}

// InitSchema ...
func (i *Impl) InitSchema() {
	i.DB.AutoMigrate(&Book{}, &User{}, &BorrowRecord{})
}

// GetAllBooks ...
func (i *Impl) GetAllBooks(w rest.ResponseWriter, r *rest.Request) {
	queryParams := r.URL.Query()
	ownerID := queryParams["ownerID"]
	borrowerID := queryParams["borrowerID"]

	query := i.DB.Preload("Owner")

	if len(borrowerID) > 0 {
		id, _ := strconv.ParseInt(borrowerID[0], 10, 64)
		fmt.Printf("borrowerID: %d\n", id)
		query = query.Preload("Borrowers", "id = ?", id).
			Joins("join book_borrowers on book_borrowers.book_id = books.id and book_borrowers.user_id = ?", id)
	} else {
		query = query.Preload("Borrowers")
	}

	if len(ownerID) > 0 {
		id, _ := strconv.ParseInt(ownerID[0], 10, 64)
		query = query.Where(&Book{OwnerID: id})
	}

	books := []Book{}
	query.Order("updated_at desc").Find(&books)

	fmt.Printf("%+v\n", books)

	w.WriteJson(&books)
}

// GetBook ...
func (i *Impl) GetBook(w rest.ResponseWriter, r *rest.Request) {
	id := r.PathParam("id")
	book := Book{}

	if i.DB.First(&book, id).Error != nil {
		rest.NotFound(w, r)
		return
	}

	users := []User{}
	i.DB.Model(&book).Related(&users, "Borrowers")
	book.Borrowers = users

	w.WriteJson(book)
}

// PostBook ...
func (i *Impl) PostBook(w rest.ResponseWriter, r *rest.Request) {
	book := Book{}
	if err := r.DecodeJsonPayload(&book); err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := i.DB.Set(
		"gorm:save_associations", false).Create(&book).Error; err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(&book)
}

// PutBook ...
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

// DeleteBook ...
func (i *Impl) DeleteBook(w rest.ResponseWriter, r *rest.Request) {
	id, _ := strconv.ParseInt(r.PathParam("id"), 10, 64)

	// check if book exist
	book := Book{}
	if i.DB.First(&book, id).Error != nil {
		rest.NotFound(w, r)
		return
	}
	fmt.Printf("delete book: %+v", &book)

	// check if book is borrowed
	count := 0
	err := i.DB.Model(&BorrowRecord{}).Where(&BorrowRecord{
		BookID: id, Status: "借阅中",
	}).Count(&count).Error

	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if count > 0 {
		rest.Error(w, "借阅中，不能删除", http.StatusInternalServerError)
		return
	}

	// delete book
	if err := i.DB.Delete(&book).Error; err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// BorrowBook borrow book
func (i *Impl) BorrowBook(w rest.ResponseWriter, r *rest.Request) {
	borrowing := Borrowing{}
	if err := r.DecodeJsonPayload(&borrowing); err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// user := borrowing.User
	// book := borrowing.Book

	user := &User{}
	book := &Book{}

	i.DB.Where(&User{ID: borrowing.User.ID}).First(&user)
	i.DB.Preload("Borrowers").Where(&Book{ID: borrowing.Book.ID}).First(&book)

	fmt.Printf("%+v", borrowing.User)
	fmt.Printf("%+v", borrowing.Book)

	startAt := time.Now()
	endAt := startAt.AddDate(0, 1, 0)

	br := BorrowRecord{
		StartAt: &startAt,
		EndAt:   &endAt,
		Status:  "借阅中",
	}

	i.DB.Set("gorm:save_associations", false).Create(&br)
	i.DB.Model(&br).Association("Book").Append(book)
	i.DB.Model(&br).Association("User").Append(user)

	i.DB.Model(&book).Association("Borrowers").Append(user)

	fmt.Printf("%+v\n", book)

	w.WriteJson(&book)
}

// ReturnBook return book
func (i *Impl) ReturnBook(w rest.ResponseWriter, r *rest.Request) {
	borrowing := Borrowing{}
	if err := r.DecodeJsonPayload(&borrowing); err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := borrowing.User
	book := borrowing.Book

	fmt.Printf("%+v", borrowing.User)
	fmt.Printf("%+v", borrowing.Book)

	br := &BorrowRecord{}
	i.DB.Where(&BorrowRecord{
		UserID: user.ID,
		BookID: book.ID,
		Status: "借阅中",
	}).First(&br)

	if br.BookID == 0 && br.UserID == 0 {
		rest.Error(w, "还书失败", http.StatusMethodNotAllowed)
		return
	}

	now := time.Now()
	i.DB.Model(&br).Update(BorrowRecord{Status: "已归还", EndAt: &now})

	i.DB.Model(&book).Association("Borrowers").Delete(user)

	fmt.Printf("%+v\n", book)

	w.WriteJson(&book)
}

// GetAllBorrowRecords ...
func (i *Impl) GetAllBorrowRecords(w rest.ResponseWriter, r *rest.Request) {
	queryParams := r.URL.Query()
	bookID := queryParams["bookID"]
	userIDs := queryParams["userIDs"]
	status := queryParams["status"]

	borrowRecord := &BorrowRecord{}
	if len(bookID) > 0 {
		id, _ := strconv.ParseInt(bookID[0], 10, 64)
		borrowRecord.BookID = id
	}
	if len(status) > 0 {
		borrowRecord.Status = status[0]
	}

	borrowRecords := []BorrowRecord{}
	// i.DB.Find(&borrowRecords)
	query := i.DB.Preload("User").Preload("Book").Where(borrowRecord)

	if len(userIDs) > 0 {
		query = query.Where("user_id in (?)", strings.Split(userIDs[0], ","))
	}

	query.Find(&borrowRecords)

	w.WriteJson(&borrowRecords)
}

// GetBorrowRecord ...
func (i *Impl) GetBorrowRecord(w rest.ResponseWriter, r *rest.Request) {
	id := r.PathParam("id")
	borrowRecord := BorrowRecord{}

	if i.DB.First(&borrowRecord, id).Error != nil {
		rest.NotFound(w, r)
		return
	}

	// users := []User{}
	// i.DB.Model(&book).Related(&users, "Borrowers")
	// book.Borrowers = users

	w.WriteJson(&borrowRecord)
}

// PostBorrowRecord ...
func (i *Impl) PostBorrowRecord(w rest.ResponseWriter, r *rest.Request) {
	borrowRecord := BorrowRecord{}

	if err := r.DecodeJsonPayload(&borrowRecord); err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("%+v", borrowRecord)

	if err := i.DB.Save(&borrowRecord).Error; err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(&borrowRecord)
}

// PutBorrowRecord ...
func (i *Impl) PutBorrowRecord(w rest.ResponseWriter, r *rest.Request) {
	id := r.PathParam("id")
	borrowRecord := BorrowRecord{}
	if i.DB.First(&borrowRecord, id).Error != nil {
		rest.NotFound(w, r)
		return
	}

	updated := BorrowRecord{}
	if err := r.DecodeJsonPayload(&updated); err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// book.Name = updated.Name
	if err := i.DB.Save(&borrowRecord).Error; err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteJson(&borrowRecord)
}

// DeleteBorrowRecord ...
func (i *Impl) DeleteBorrowRecord(w rest.ResponseWriter, r *rest.Request) {
	id := r.PathParam("id")
	borrowRecord := BorrowRecord{}
	if i.DB.First(&borrowRecord, id).Error != nil {
		rest.NotFound(w, r)
		return
	}
	if err := i.DB.Delete(&borrowRecord).Error; err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetAllUsers ...
func (i *Impl) GetAllUsers(w rest.ResponseWriter, r *rest.Request) {
	queryParams := r.URL.Query()
	name := queryParams["name"]
	email := queryParams["email"]

	user := &User{}
	if len(name) > 0 {
		user.Name = name[0]
	}
	if len(email) > 0 {
		user.Email = email[0]
	}

	users := []User{}
	// i.DB.Find(&borrowRecords)
	i.DB.Where(user).Find(&users)

	w.WriteJson(&users)
}

// handleAuth ...
func handleAuth(w rest.ResponseWriter, r *rest.Request) {
	w.WriteJson(map[string]string{"authed": r.Env["REMOTE_USER"].(string)})
}

func (i *Impl) importBooks() {
	csvfile, err := os.Open("local/books.csv")

	if err != nil {
		fmt.Println(err)
		return
	}

	defer csvfile.Close()

	reader := csv.NewReader(csvfile)

	reader.FieldsPerRecord = -1 // see the Reader struct information below

	rawCSVdata, err := reader.ReadAll()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// sanity check, display to standard output
	fmt.Printf("total size: %d", len(rawCSVdata))
	for _, each := range rawCSVdata {
		fmt.Printf("Name : %s , Author : %s and Quantity: %s\n",
			each[0], each[1], each[2])
		quantity, err := strconv.Atoi(each[2])

		if err != nil {
			quantity = 1
		}

		book := Book{
			Name:     each[0],
			Author:   each[1],
			Quantity: quantity,
		}
		i.DB.Set("gorm:save_associations", false).Create(&book)

		admin := User{}
		i.DB.Where(&User{Name: "admin"}).First(&admin)
		i.DB.Model(&book).Association("Owner").Append(admin)
	}
}

func (i *Impl) importUsers() {
	csvfile, err := os.Open("local/users.csv")

	if err != nil {
		fmt.Println(err)
		return
	}

	defer csvfile.Close()

	reader := csv.NewReader(csvfile)

	reader.FieldsPerRecord = -1 // see the Reader struct information below

	rawCSVdata, err := reader.ReadAll()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// sanity check, display to standard output
	fmt.Printf("total size: %d", len(rawCSVdata))
	for _, each := range rawCSVdata {
		fmt.Printf("Name : %s , Email : %s\n",
			each[0], each[1])

		user := User{
			Name:     each[0],
			Email:    each[1],
			Password: "passw0rd",
		}
		i.DB.Set("gorm:save_associations", false).Create(&user)
	}

	admin := User{
		Name:     "admin",
		Email:    "admin",
		Password: "admin1378^",
	}
	i.DB.Set("gorm:save_associations", false).Create(&admin)
}

func (i *Impl) connectUsersAndBooks() {
	csvfile, err := os.Open("local/records.csv")

	if err != nil {
		fmt.Println(err)
		return
	}

	defer csvfile.Close()

	reader := csv.NewReader(csvfile)

	reader.FieldsPerRecord = -1 // see the Reader struct information below

	rawCSVdata, err := reader.ReadAll()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// sanity check, display to standard output
	fmt.Printf("total size: %d\n", len(rawCSVdata))
	for _, each := range rawCSVdata {
		// fmt.Printf("Name : %s , User : %s, date : %s\n",
		// 	each[0], each[1], each[2])

		startAt := time.Date(2016, time.August, 15, 0, 0, 0, 0, time.UTC)
		br := BorrowRecord{
			StartAt: &startAt,
			Status:  "借阅中",
		}

		book := Book{}
		i.DB.Where(&Book{Name: each[0]}).First(&book)
		// if book.Name == "" {
		// 	fmt.Printf("book not found: %s\n", each[0])
		// }

		user := User{}
		i.DB.Where(&User{Name: each[1]}).First(&user)
		// if user.Name == "" {
		// 	fmt.Printf("user not found: %s\n", each[1])
		// }

		i.DB.Set("gorm:save_associations", false).Create(&br)
		i.DB.Model(&br).Association("Book").Append(book)
		i.DB.Model(&br).Association("User").Append(user)

		i.DB.Model(&book).Association("Borrowers").Append(user)
		fmt.Printf("%+v\n", br)
	}
}

func (i *Impl) deleteBR() {
	count := 0

	i.DB.Delete(&BorrowRecord{})
	i.DB.Unscoped().Delete(&BorrowRecord{})
	i.DB.Model(&BorrowRecord{}).Count(&count)
	fmt.Printf("remain br: %d\n", count)
}

// import data from csv
func (i *Impl) importFromCsv() {
	// i.importUsers()
	// i.importBooks()
	//
	// i.connectUsersAndBooks()

	// i.deleteBR()

	// book := Book{
	// 	Name:      "webGL",
	// 	Borrowers: []User{{Name: "范晓", Password: "123456", Email: "fanxiao@k2data.com.cn"}},
	// }
	// i.DB.Create(&book)

	// book := Book{}
	// if i.DB.First(&book, 1).Error != nil {
	//
	// }
	//
	// user := User{}
	// if i.DB.First(&user, 1).Error != nil {
	// }
	// fmt.Printf("%+v", user)
	//
	// borrowRecord := BorrowRecord{
	// 	StartAt: time.Now(),
	// 	EndAt:   time.Now(),
	// 	Book:    book,
	// 	User:    user,
	// }
	// i.DB.Create(&borrowRecord)
}

func (i *Impl) validateUser(email string, password string) bool {
	count := 0
	i.DB.Model(&User{}).
		Where("email = ? and password = ?", email, password).
		Count(&count)
	fmt.Printf("%d\n", count)
	return count == 1 || (email == "admin" && password == "admin1378^")
}

// main ...
func main() {

	i := Impl{}
	i.InitDB()
	i.InitSchema()
	i.importFromCsv()

	jwtMiddleware := &jwt.JWTMiddleware{
		Key:        []byte("secret key"),
		Realm:      "jwt auth",
		Timeout:    time.Hour * 12,
		MaxRefresh: time.Hour * 24,
		Authenticator: func(userId string, password string) bool {
			return i.validateUser(userId, password)
		}}

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
		IfTrue: jwtMiddleware,
	})

	router, err := rest.MakeRouter(
		rest.Post("/login", jwtMiddleware.LoginHandler),
		rest.Get("/auth_test", handleAuth),
		rest.Get("/refresh_token", jwtMiddleware.RefreshHandler),
		rest.Get("/books", i.GetAllBooks),
		rest.Post("/books", i.PostBook),
		rest.Post("/books/borrow", i.BorrowBook),
		rest.Post("/books/return", i.ReturnBook),
		rest.Get("/books/:id", i.GetBook),
		rest.Put("/books/:id", i.PutBook),
		rest.Delete("/books/:id", i.DeleteBook),
		rest.Get("/borrow-records", i.GetAllBorrowRecords),
		rest.Post("/borrow-records", i.PostBorrowRecord),
		rest.Get("/borrow-records/:id", i.GetBorrowRecord),
		rest.Put("/borrow-records/:id", i.PutBorrowRecord),
		rest.Delete("/borrow-records/:id", i.DeleteBorrowRecord),
		rest.Get("/users", i.GetAllUsers),
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
