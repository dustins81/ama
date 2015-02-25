package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	_ "github.com/lib/pq"
	"go-amazon-product-api"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var db *sql.DB

func amaCred() amazonproduct.AmazonProductAPI {
	var api amazonproduct.AmazonProductAPI
	api.AccessKey = "AKIAINDXDOD46W3WTAOA"
	api.SecretKey = "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"
	api.AssociateTag = "honeymerch-20"
	api.Host = "webservices.amazon.com"
	return api
}

type Page struct {
	Title string
	Link  string
	Pic   string
}
type Search struct {
	XMLName xml.Name `xml:"ItemSearchResponse"`
	Items   Items    `xml:"Items"`
}
type Items struct {
	Products []Product `xml:"Item"`
}
type Product struct {
	ASIN  string `xml:"ASIN"`
	Price string `xml:"ItemAttributes>ListPrice>FormattedPrice"`
	URL   string `xml:"DetailPageURL"`
	Image string `xml:"MediumImage>URL"`
	Title string `xml:"ItemAttributes>Title"`
}
type Item struct {
	XMLName xml.Name `xml:"ItemLookupResponse"`
	ASIN    string   `xml:"Items>Item>ASIN"`
	Price   string   `xml:"Items>Item>ItemAttributes>ListPrice>FormattedPrice"`
	URL     string   `xml:"Items>Item>DetailPageURL"`
	Image   string   `xml:"Items>Item>MediumImage>URL"`
	Title   string   `xml:"Items>Item>ItemAttributes>Title"`
	Cat     string
}

var templates = template.Must(template.ParseFiles("header.html", "index.html", "category.html", "edit.html", "footer.html"))

func display(w http.ResponseWriter, tmpl string, data interface{}) {
	templates.ExecuteTemplate(w, tmpl, data)
}
func difference(slice1 []string, slice2 []string) []string {
	var diff []string

	// Loop two times, first to find slice1 strings not in slice2,
	// second loop to find slice2 strings not in slice1
	for i := 0; i < 2; i++ {
		for _, s1 := range slice1 {
			found := false
			for _, s2 := range slice2 {
				if s1 == s2 {
					found = true
					break
				}
			}
			// String not found. We add it to return slice
			if !found {
				diff = append(diff, s1)
			}
		}
		// Swap the slices, only if it was the first loop
		if i == 0 {
			slice1, slice2 = slice2, slice1
		}
	}

	return diff
}
func SetupDB() *sql.DB {
	db, err := sql.Open("postgres", "user=gorilla dbname=products sslmode=disable")
	if err != nil {
		fmt.Println(err)
	}
	return db
}
func dbQuery(q string) (*sql.Rows, error) {
	db := SetupDB()
	return db.Query(q)
}

func CatReturn() []string {
	rows, err := dbQuery("SELECT title FROM category ORDER BY title ASC")
	if err != nil {
		fmt.Println(err)
	}
	defer rows.Close()
	vals := make([]string, 0)
	var title string
	for rows.Next() {
		err := rows.Scan(&title)
		if err != nil {
			fmt.Println(err)
		}
		vals = append(vals, title)
	}
	return (vals)
}

func amaSearch(w http.ResponseWriter, r *http.Request) {
	api := amaCred()
	var form_data string
	data := make(map[string]interface{})
	if r.Method == "POST" {
		var v Search
		form_data = r.FormValue("search")
		result, err := api.ItemSearchByKeyword(url.QueryEscape(form_data), 1)
		if err != nil {
			fmt.Println(err)
		}
		err = xml.Unmarshal([]byte(result), &v)
		if err != nil {
			fmt.Printf("errr: %v", err)
			return
		}
		data["vals"] = v
	}

	cats := CatReturn()
	data["cats"] = cats
	//	t, _ := template.ParseFiles("index.html")
	//	t.Execute(w, data)
	display(w, "index", data)
}
func createCat(w http.ResponseWriter, r *http.Request) {
	var form_data string
	if r.Method == "POST" && r.FormValue("category") != "" {
		form_data = r.FormValue("category")
		_, err := db.Exec("INSERT INTO category (title) VALUES($1)", form_data)
		if err != nil {
			fmt.Println(err)
		}
	}
	cats := CatReturn()
	display(w, "category", cats)
}
func deleteCat(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
	} else {
		fmt.Println("No POST")
	}
	u := r.Form
	d := u["category"]
	for i := range d {
		_, err := db.Exec("DELETE FROM category WHERE title = $1", d[i])
		if err != nil {
			fmt.Println(err)
		}
	}
	http.Redirect(w, r, "/category/", 301)
}
func prodSave(w http.ResponseWriter, r *http.Request) {
	api := amaCred()
	if r.Method == "POST" {
		r.ParseForm()
	}
	var v Item
	u := r.Form
	d := u["products"]
	e := u["catsel"]
	fmt.Println(e)
	for j := range d {
		result, err := api.ItemLookup(d[j])
		if err != nil {
			fmt.Println("err: %v", err)
		}
		err = xml.Unmarshal([]byte(result), &v)
		if err != nil {
			fmt.Printf("err %v", err)
		}
		_, err = db.Exec(`INSERT INTO product (asin, price, url, image, title, category) VALUES ($1,$2,$3,$4,$5,$6)`, v.ASIN, v.Price, v.URL, v.Image, v.Title, e[0])
		if err != nil {
			fmt.Println(err)
		}
	}
	http.Redirect(w, r, "/", 301)
}
func prodEdit(w http.ResponseWriter, r *http.Request) {
	var rows *sql.Rows
	var err error
	if r.Method == "POST" {
		r.ParseForm()
		u := r.Form
		f := make([]string, 0)
		d := u["category"]
		if d == nil {
			d = CatReturn()
		}
		for h := range d {
			s := strconv.Quote(d[h])
			g := strings.Replace(s, "\"", `'`, 5)
			f = append(f, g)
		}
		e := strings.Join(f, " OR category=")
		rows, err = dbQuery("SELECT asin, price, url, image, title, category FROM product WHERE category=" + e)
		if err != nil {
			fmt.Println(err)
		}

	} else {
		fmt.Println("else")
		rows, err = dbQuery("SELECT asin, price, url, image, title, category FROM product")
		if err != nil {
			fmt.Println(err)
		}
	}

	var T Item
	prods := make([]Item, 0)
	for rows.Next() {
		err := rows.Scan(&T.ASIN, &T.Price, &T.URL, &T.Image, &T.Title, &T.Cat)

		if err != nil {
			fmt.Println(err)
		}
		prods = append(prods, T)
	}
	data := make(map[string]interface{})
	data["prods"] = prods
	data["cats"] = CatReturn()
	//t, _ := template.ParseFiles("edit.html")
	//t.Execute(w, data)
	display(w, "edit", data)
}
func catedit(w http.ResponseWriter, r *http.Request) {
	var id, val string
	if r.Method == "POST" {
		id = r.FormValue("id")
		val = r.FormValue("catsel")
	}
	fmt.Println(val)
	fmt.Println(id)
	_, err := db.Exec("UPDATE product SET category = $1 WHERE asin = $2", val, id)
	if err != nil {
		fmt.Println(err)
	}
	http.Redirect(w, r, "/edit/", 301)

}
func prodsDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
	} else {
		fmt.Println("No POST")
	}
	u := r.Form
	d := u["product"]
	for i := range d {
		_, err := db.Exec("DELETE FROM product WHERE ASIN = $1", d[i])
		if err != nil {
			fmt.Println(err)
		}
	}
	http.Redirect(w, r, "/edit/", 301)
}
func main() {
	db = SetupDB()
	defer db.Close()
	http.HandleFunc("/", amaSearch)
	http.HandleFunc("/save/", prodSave)
	http.HandleFunc("/edit/", prodEdit)
	http.HandleFunc("/update/", prodsDelete)
	http.HandleFunc("/category/", createCat)
	http.HandleFunc("/catedit/", catedit)
	http.HandleFunc("/catdel/", deleteCat)
	http.HandleFunc("/proddel/", prodsDelete)
	http.ListenAndServe("127.0.0.1:8080", nil)
}
