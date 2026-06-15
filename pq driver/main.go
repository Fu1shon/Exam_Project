package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

type User struct {
	ID    int
	Login string
	FIO   string
	Phone string
	Email string
}

type Booking struct {
	ID         int
	UserID     int
	Room       string
	Date       string
	Payment    string
	Status     string
	Review     string
	ReviewDate string
}

var db *sql.DB
var tpl *template.Template

func main() {
	connStr := "host=localhost user=postgres password=0706 dbname=test_db sslmode=disable"
	db, _ = sql.Open("postgres", connStr)
	db.Ping()

	tpl = template.Must(template.ParseGlob("templates/*.html"))

	db.Exec(`CREATE TABLE IF NOT EXISTS users(
		id SERIAL PRIMARY KEY,
		login TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		fio TEXT NOT NULL,
		phone TEXT NOT NULL,
		email TEXT NOT NULL
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS bookings(
		id SERIAL PRIMARY KEY,
		userid INTEGER REFERENCES users(id),
		room TEXT NOT NULL,
		date TEXT NOT NULL,
		payment TEXT NOT NULL,
		status TEXT DEFAULT 'Новая',
		review TEXT,
		review_date TEXT
	)`)

	var cnt int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE login='Admin26'").Scan(&cnt)
	if cnt == 0 {
		db.Exec("INSERT INTO users(login, password, fio, phone, email) VALUES('Admin26','Demo20','Админ','+7(000)-000-00-00','admin@b.ru')")
	}

	http.HandleFunc("/", index)
	http.HandleFunc("/register", register)
	http.HandleFunc("/login", login)
	http.HandleFunc("/logout", logout)
	http.HandleFunc("/profile", profile)
	http.HandleFunc("/new-booking", newBooking)
	http.HandleFunc("/admin", admin)
	http.HandleFunc("/review", review)

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("js"))))

	fmt.Println("Сервер открыт на http://localhost:8181/")
	http.ListenAndServe(":8181", nil)
}

func index(w http.ResponseWriter, r *http.Request) {
	tpl.ExecuteTemplate(w, "index.html", nil)
}

func register(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tpl.ExecuteTemplate(w, "register.html", nil)
		return
	}

	login := r.FormValue("login")
	password := r.FormValue("password")
	fio := r.FormValue("fio")
	phone := r.FormValue("phone")
	email := r.FormValue("email")

	_, err := db.Exec("INSERT INTO users(login, password, fio, phone, email) VALUES($1,$2,$3,$4,$5)",
		login, password, fio, phone, email)

	if err != nil {
		tpl.ExecuteTemplate(w, "register.html", "Логин занят")
		return
	}
	http.Redirect(w, r, "/login", 302)
}

func login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tpl.ExecuteTemplate(w, "login.html", nil)
		return
	}

	login := r.FormValue("login")
	password := r.FormValue("password")

	var id int
	var userLogin string
	err := db.QueryRow("SELECT id, login FROM users WHERE login=$1 AND password=$2", login, password).Scan(&id, &userLogin)

	if err != nil {
		tpl.ExecuteTemplate(w, "login.html", "Неверный логин или пароль")
		return
	}

	http.SetCookie(w, &http.Cookie{Name: "user_id", Value: strconv.Itoa(id), Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "user_login", Value: userLogin, Path: "/"})

	if userLogin == "Admin26" {
		http.Redirect(w, r, "/admin", 302)
	} else {
		http.Redirect(w, r, "/profile", 302)
	}
}

func logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "user_id", Value: "", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "user_login", Value: "", MaxAge: -1})
	http.Redirect(w, r, "/", 302)
}

func getUserID(r *http.Request) int {
	c, _ := r.Cookie("user_id")
	if c == nil {
		return 0
	}
	id, _ := strconv.Atoi(c.Value)
	return id
}

func getUserLogin(r *http.Request) string {
	c, _ := r.Cookie("user_login")
	if c == nil {
		return ""
	}
	return c.Value
}

func profile(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		http.Redirect(w, r, "/login", 302)
		return
	}

	var user User
	user.ID = userID
	user.Login = getUserLogin(r)
	db.QueryRow("SELECT fio, phone, email FROM users WHERE id=$1", userID).Scan(&user.FIO, &user.Phone, &user.Email)

	rows, _ := db.Query("SELECT id, room, date, payment, status FROM bookings WHERE userid=$1 ORDER BY date DESC", userID)
	defer rows.Close()

	var bookings []Booking
	for rows.Next() {
		var b Booking
		b.UserID = userID
		rows.Scan(&b.ID, &b.Room, &b.Date, &b.Payment, &b.Status)
		bookings = append(bookings, b)
	}

	tpl.ExecuteTemplate(w, "profile.html", map[string]interface{}{
		"User": user,
		"List": bookings,
	})
}

func newBooking(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		http.Redirect(w, r, "/login", 302)
		return
	}

	if r.Method == "GET" {
		tpl.ExecuteTemplate(w, "new-booking.html", nil)
		return
	}

	room := r.FormValue("room")
	date := r.FormValue("date")
	payment := r.FormValue("payment")

	db.Exec("INSERT INTO bookings(userid, room, date, payment) VALUES($1,$2,$3,$4)",
		userID, room, date, payment)

	http.Redirect(w, r, "/profile", 302)
}

func admin(w http.ResponseWriter, r *http.Request) {
	if getUserLogin(r) != "Admin26" {
		http.Redirect(w, r, "/", 302)
		return
	}

	if r.Method == "POST" {
		status := r.FormValue("status")
		id := r.FormValue("id")
		db.Exec("UPDATE bookings SET status=$1 WHERE id=$2", status, id)
		http.Redirect(w, r, "/admin", 302)
		return
	}

	// Фильтр
	filter := r.URL.Query().Get("status")
	query := "SELECT id, userid, room, date, payment, status FROM bookings"
	if filter != "" && filter != "Все" {
		query += " WHERE status='" + filter + "'"
	}
	query += " ORDER BY date DESC"

	rows, _ := db.Query(query)
	defer rows.Close()

	var bookings []Booking
	for rows.Next() {
		var b Booking
		rows.Scan(&b.ID, &b.UserID, &b.Room, &b.Date, &b.Payment, &b.Status)
		bookings = append(bookings, b)
	}

	tpl.ExecuteTemplate(w, "admin.html", map[string]interface{}{
		"List":   bookings,
		"Filter": filter,
	})
}

func review(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		http.Redirect(w, r, "/login", 302)
		return
	}

	reviewText := r.FormValue("review")
	id := r.FormValue("id")

	db.Exec("UPDATE bookings SET review=$1, review_date=$2 WHERE id=$3 AND userid=$4 AND status='Банкет завершен'",
		reviewText, time.Now().Format("02.01.2006"), id, userID)

	http.Redirect(w, r, "/profile", 302)
}
