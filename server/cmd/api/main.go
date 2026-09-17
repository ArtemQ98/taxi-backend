package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	// "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db        *pgxpool.Pool
	jwtSecret []byte
}

type User struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email,omitempty"`
	Phone         string `json:"phone,omitempty"`
	PhoneVerified bool   `json:"phone_verified"`
	Role          string `json:"role"`
	City          string `json:"city"`
	CompanyName   string `json:"company_name"`
}

type Car struct {
	ID            int64   `json:"id"`
	Brand         string  `json:"brand"`
	Model         string  `json:"model"`
	Plate         string  `json:"plate"`
	Year          int     `json:"year"`
	Status        string  `json:"status"`
	Revenue       float64 `json:"revenue"`
	Location      string  `json:"location"`
	DailyPrice    float64 `json:"daily_price"`
	Mileage       int     `json:"mileage"`
	Color         string  `json:"color"`
	VIN           string  `json:"vin"`
	PublicEnabled bool    `json:"public_enabled"`
	Category      string  `json:"category"`
	Seats         int     `json:"seats"`
	Transmission  string  `json:"transmission"`
	Fuel          string  `json:"fuel"`
	Description   string  `json:"description"`
	ImageURL      string  `json:"image_url"`
	Deposit       float64 `json:"deposit"`
	EngineVolume  string `json:"engine_volume"`
	Horsepower    int `json:"horsepower"`
	DriveType     string `json:"drive_type"`
	FuelConsumption string `json:"fuel_consumption"`
	TankVolume    string `json:"tank_volume"`
	MaintenanceInterval int `json:"maintenance_interval"`
	Earnings      float64 `json:"earnings"`
	Expenses      float64 `json:"expenses"`
}


func (a *App) taxiProfile(w http.ResponseWriter, r *http.Request) {
    id := userID(r.Context())
    if r.Method == "PATCH" {
        var in struct {
            Name string `json:"name"`
            Phone string `json:"phone"`
            City string `json:"city"`
            CarBrand string `json:"car_brand"`
            CarModel string `json:"car_model"`
            Plate string `json:"plate"`
        }
        if decode(r, &in) != nil {
            write(w, 400, map[string]string{"error":"invalid json"})
            return
        }
        in.Name = strings.TrimSpace(in.Name)
        in.Phone = normalizePhone(strings.TrimSpace(in.Phone))
        in.City = strings.TrimSpace(in.City)
        in.CarBrand = strings.TrimSpace(in.CarBrand)
        in.CarModel = strings.TrimSpace(in.CarModel)
        in.Plate = strings.ToUpper(strings.TrimSpace(in.Plate))
        if in.Name == "" || len(in.Phone) < 12 || in.City == "" || in.CarBrand == "" || in.CarModel == "" || in.Plate == "" {
            write(w, 422, map[string]string{"error":"заполните имя, телефон, город, марку, модель и госномер"})
            return
        }
        _, err := a.db.Exec(r.Context(), `
            UPDATE users SET name=$1, phone=$2, city=$3, taxi_city=$3,
            taxi_car_brand=$4, taxi_car_model=$5, taxi_plate=$6, taxi_updated_at=now()
            WHERE id=$7`, in.Name, in.Phone, in.City, in.CarBrand, in.CarModel, in.Plate, id)
        if err != nil {
            write(w, 409, map[string]string{"error":"не удалось сохранить профиль"})
            return
        }
    }
    var p map[string]any
    var name, phone, city, brand, model, plate, status string
    err := a.db.QueryRow(r.Context(), `
        SELECT name,COALESCE(phone,''),COALESCE(taxi_city,city,''),COALESCE(taxi_car_brand,''),
        COALESCE(taxi_car_model,''),COALESCE(taxi_plate,''),COALESCE(taxi_status,'busy')
        FROM users WHERE id=$1`, id).Scan(&name,&phone,&city,&brand,&model,&plate,&status)
    if err != nil {
        write(w, 404, map[string]string{"error":"водитель не найден"})
        return
    }
    p = map[string]any{"id":id,"name":name,"phone":phone,"city":city,"car_brand":brand,"car_model":model,"plate":plate,"status":status}
    write(w, 200, p)
}

func (a *App) taxiStatus(w http.ResponseWriter, r *http.Request) {
    if r.Method != "PATCH" {
        write(w, 405, map[string]string{"error":"method not allowed"})
        return
    }
    var in struct{ Status string `json:"status"` }
    if decode(r, &in) != nil || (in.Status != "available" && in.Status != "busy") {
        write(w, 422, map[string]string{"error":"status must be available or busy"})
        return
    }
    id := userID(r.Context())
    _, err := a.db.Exec(r.Context(), `UPDATE users SET taxi_status=$1,taxi_updated_at=now() WHERE id=$2`, in.Status, id)
    if err != nil {
        write(w, 500, map[string]string{"error":"не удалось изменить статус"})
        return
    }

    var name, phone, city, brand, model, plate, status string
    err = a.db.QueryRow(r.Context(), `
        SELECT name,COALESCE(phone,''),COALESCE(taxi_city,city,''),COALESCE(taxi_car_brand,''),
        COALESCE(taxi_car_model,''),COALESCE(taxi_plate,''),COALESCE(taxi_status,'busy')
        FROM users WHERE id=$1`, id).Scan(&name,&phone,&city,&brand,&model,&plate,&status)
    if err != nil {
        write(w, 404, map[string]string{"error":"водитель не найден"})
        return
    }
    write(w, 200, map[string]any{
        "id":id,"name":name,"phone":phone,"city":city,
        "car_brand":brand,"car_model":model,"plate":plate,"status":status,
    })
}

func (a *App) publicTaxiDrivers(w http.ResponseWriter, r *http.Request) {
    city := strings.TrimSpace(r.URL.Query().Get("city"))
    if city == "" {
        write(w, 422, map[string]string{"error":"укажите город или посёлок"})
        return
    }
    rows, err := a.db.Query(r.Context(), `
        SELECT id,name,phone,taxi_city,taxi_car_brand,taxi_car_model,taxi_plate
        FROM users
        WHERE role='owner' AND taxi_status='available'
          AND taxi_city ILIKE '%'||$1||'%'
          AND taxi_car_brand <> '' AND taxi_car_model <> '' AND taxi_plate <> ''
        ORDER BY name`, city)
    if err != nil {
        write(w, 500, map[string]string{"error":err.Error()})
        return
    }
    defer rows.Close()
    out := []map[string]any{}
    for rows.Next() {
        var id int64
        var name, phone, fcity, brand, model, plate string
        if rows.Scan(&id,&name,&phone,&fcity,&brand,&model,&plate) == nil {
            out = append(out, map[string]any{
                "id":id,"name":name,"phone":phone,"city":fcity,
                "car_brand":brand,"car_model":model,"plate":plate,
                "status":"available",
            })
        }
    }
    write(w, 200, out)
}

func main() {
	ctx := context.Background()
	dsn := getenv("DATABASE_URL", "postgres://key:key@localhost:5432/key?sslmode=disable")
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		log.Fatal("database: ", err)
	}
	if err := ensureBaseSchema(ctx, db); err != nil {
    	log.Fatal("base migration: ", err)
	}
	if err := ensureTaxiSchema(ctx, db); err != nil {
		log.Fatal("taxi migration: ", err)
	}
	if err := ensureMarketplaceSchema(ctx, db); err != nil {
		log.Fatal("marketplace migration: ", err)
	}
	if err := ensureRentalCoreSchema(ctx, db); err != nil {
		log.Fatal("rental core migration: ", err)
	}
	if err := ensureRentalFinanceSchema(ctx, db); err != nil {
		log.Fatal("rental finance migration: ", err)
	}
	if err := ensureCarPhotosSchema(ctx, db); err != nil {
		log.Fatal("car photos migration: ", err)
	}
	if err := ensureBookingUXSchema(ctx, db); err != nil {
		log.Fatal("booking ux migration: ", err)
	}
	if err := ensureCarFinanceSchema(ctx, db); err != nil {
		log.Fatal("car finance migration: ", err)
	}
	if err := ensurePhoneAuthSchema(ctx, db); err != nil {
		log.Fatal("phone auth migration: ", err)
	}
	if err := ensureFleetExtendedSchema(ctx, db); err != nil {
		log.Fatal("fleet extended migration: ", err)
	}
	if err := os.MkdirAll("/app/uploads/cars", 0o755); err != nil {
		log.Fatal("uploads: ", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" || jwtSecret == "secret" || jwtSecret == "change-me-in-production" {
		log.Fatal("JWT_SECRET не задан или слишком простой — задайте надёжный секрет в env")
	}
	app := &App{db: db, jwtSecret: []byte(jwtSecret)}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", app.health)
	mux.HandleFunc("/api/auth/register", app.register)
	mux.HandleFunc("/api/auth/login", app.login)
	mux.HandleFunc("/api/auth/customer/register", app.customerRegister)
	mux.HandleFunc("/api/auth/customer/login", app.customerLogin)
	mux.HandleFunc("/api/auth/request-code", app.requestCode)
	mux.HandleFunc("/api/auth/verify-code", app.verifyCode)
	mux.HandleFunc("/api/public/taxi-drivers", app.publicTaxiDrivers)
	mux.HandleFunc("/api/public/fleets", app.publicFleets)
	mux.HandleFunc("/api/public/cars", app.publicCars)
	mux.HandleFunc("/api/public/cars/", app.publicCar)
	mux.HandleFunc("/api/public/availability", app.publicAvailability)
	mux.HandleFunc("/api/public/availability/dates", app.publicAvailabilityDates)
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("/app/uploads"))))
	mux.Handle("/api/taxi/profile", app.ownerOnly(http.HandlerFunc(app.taxiProfile)))
	mux.Handle("/api/taxi/status", app.ownerOnly(http.HandlerFunc(app.taxiStatus)))
	mux.Handle("/api/me", app.auth(http.HandlerFunc(app.me)))
	mux.Handle("/api/profile", app.ownerOnly(http.HandlerFunc(app.profile)))
	mux.Handle("/api/fleet-profile", app.ownerOnly(http.HandlerFunc(app.fleetProfile)))
	mux.Handle("/api/dashboard", app.ownerOnly(http.HandlerFunc(app.dashboard)))
	mux.Handle("/api/cars", app.ownerOnly(http.HandlerFunc(app.cars)))
	mux.Handle("/api/cars/", app.ownerOnly(http.HandlerFunc(app.carByID)))
	mux.Handle("/api/car-finance/", app.ownerOnly(http.HandlerFunc(app.carFinance)))
	mux.Handle("/api/car-photos/", app.ownerOnly(http.HandlerFunc(app.carPhotos)))
	mux.Handle("/api/rentals", app.ownerOnly(http.HandlerFunc(app.rentals)))
	mux.Handle("/api/rentals/calendar", app.ownerOnly(http.HandlerFunc(app.rentalCalendar)))
	mux.Handle("/api/rentals/", app.ownerOnly(http.HandlerFunc(app.rentalByID)))
	mux.Handle("/api/rental-ops/", app.ownerOnly(http.HandlerFunc(app.rentalOps)))
	mux.Handle("/api/rentals/meeting/", app.ownerOnly(http.HandlerFunc(app.rentalMeeting)))
	mux.Handle("/api/rentals/payment/", app.ownerOnly(http.HandlerFunc(app.rentalPayment)))
	mux.Handle("/api/verification", app.ownerOnly(http.HandlerFunc(app.verification)))
	mux.Handle("/api/verification/", app.ownerOnly(http.HandlerFunc(app.verificationByID)))
	mux.Handle("/api/clients", app.ownerOnly(http.HandlerFunc(app.clients)))
	mux.Handle("/api/notifications", app.ownerOnly(http.HandlerFunc(app.notifications)))
	mux.Handle("/api/bookings", app.auth(http.HandlerFunc(app.bookings)))
	mux.Handle("/api/bookings/", app.auth(http.HandlerFunc(app.bookingByID)))
	mux.Handle("/api/customer/bookings", app.auth(http.HandlerFunc(app.customerBookings)))
	mux.HandleFunc("/api/leads", http.HandlerFunc(app.leads))

	port := getenv("PORT", "8080")
	log.Printf("KEY API :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, cors(logging(mux))))
}

func ensureBaseSchema(ctx context.Context, db *pgxpool.Pool) error {
    // 001_init создаёт users и базовые таблицы. Проверяем по users.
    var exists bool
    if err := db.QueryRow(ctx, `SELECT to_regclass('public.users') IS NOT NULL`).Scan(&exists); err != nil {
        return err
    }
    if exists {
        return nil
    }
    // 002_seed пустой; 003_product дублирует хвост 001_init — оба не нужны.
    b, err := os.ReadFile(filepath.Join("/app", "migrations", "001_init.sql"))
    if err != nil {
        return fmt.Errorf("001_init.sql: %w", err)
    }
    if _, err := db.Exec(ctx, string(b)); err != nil {
        return fmt.Errorf("001_init.sql: %w", err)
    }
    log.Println("KEY migration applied: 001_init.sql")
    return nil
}

func ensureTaxiSchema(ctx context.Context, db *pgxpool.Pool) error {
    path := filepath.Join("/app", "migrations", "013_taxi.sql")
    b, err := os.ReadFile(path)
    if err != nil { return err }
    _, err = db.Exec(ctx, string(b))
    return err
}

func ensureMarketplaceSchema(ctx context.Context, db *pgxpool.Pool) error {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.fleet_profiles') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join("/app", "migrations", "004_marketplace.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, string(b)); err != nil {
		return err
	}
	log.Println("KEY marketplace schema applied")
	return nil
}

func ensureRentalCoreSchema(ctx context.Context, db *pgxpool.Pool) error {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.rental_events') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	b, err := os.ReadFile(filepath.Join("/app", "migrations", "005_rental_core.sql"))
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, string(b)); err != nil {
		return err
	}
	log.Println("KEY rental core schema applied")
	return nil
}

func ensureRentalFinanceSchema(ctx context.Context, db *pgxpool.Pool) error {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.rental_adjustments') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	b, err := os.ReadFile(filepath.Join("/app", "migrations", "006_rental_finance.sql"))
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, string(b)); err != nil {
		return err
	}
	log.Println("KEY rental finance schema applied")
	return nil
}

func ensurePhoneAuthSchema(ctx context.Context, db *pgxpool.Pool) error {
	b, err := os.ReadFile(filepath.Join("/app", "migrations", "010_phone_auth.sql"))
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, string(b))
	return err
}


func ensureFleetExtendedSchema(ctx context.Context, db *pgxpool.Pool) error {
	b, err := os.ReadFile(filepath.Join("/app", "migrations", "011_fleet_profiles_extended.sql"))
	if err != nil { return err }
	_, err = db.Exec(ctx, string(b))
	return err
}

func ensureCarFinanceSchema(ctx context.Context, db *pgxpool.Pool) error {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.car_expenses') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	b, err := os.ReadFile(filepath.Join("/app", "migrations", "009_car_finance.sql"))
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, string(b)); err != nil {
		return err
	}
	log.Println("KEY car finance schema applied")
	return nil
}

func ensureBookingUXSchema(ctx context.Context, db *pgxpool.Pool) error {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='rentals' AND column_name='pickup_meeting_at')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	b, err := os.ReadFile(filepath.Join("/app", "migrations", "008_booking_ux.sql"))
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, string(b)); err != nil {
		return err
	}
	log.Println("KEY booking UX schema applied")
	return nil
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }
func (a *App) health(w http.ResponseWriter, r *http.Request) {
	if err := a.db.Ping(r.Context()); err != nil {
		write(w, 503, map[string]any{"ok": false, "error": "db unavailable"})
		return
	}
	write(w, 200, map[string]any{"ok": true, "db": "postgresql"})
}

func normalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	n := b.String()
	if strings.HasPrefix(n, "8") && len(n) == 11 {
		n = "7" + n[1:]
	}
	if strings.HasPrefix(n, "7") && len(n) == 11 {
		return "+" + n
	}
	return s
}

func validEmail(s string) bool { return strings.Contains(s, "@") && strings.Contains(s, ".") }

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, Phone, Email, Password, City, CompanyName string }
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Phone = normalizePhone(strings.TrimSpace(in.Phone))
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Name == "" || len(in.Password) < 8 || len(in.Phone) < 12 {
		write(w, 422, map[string]string{"error": "имя, телефон и пароль (8+ символов) обязательны"})
		return
	}
	if in.Email != "" && !validEmail(in.Email) {
		write(w, 422, map[string]string{"error": "некорректный email"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	var id int64
	err := a.db.QueryRow(r.Context(), `
		INSERT INTO users(name,phone,email,password_hash,role,city,company_name)
		VALUES($1,$2,NULLIF($3,''),$4,'owner',$5,$6)
		RETURNING id`, in.Name, in.Phone, in.Email, string(hash), first(in.City, "Санкт-Петербург"), strings.TrimSpace(in.CompanyName)).Scan(&id)
	if err != nil {
		write(w, 409, map[string]string{"error": "телефон или email уже зарегистрирован"})
		return
	}
	// Every new owner gets a published storefront immediately, so their cars can
	// appear in the customer Marketplace without an extra setup step.
	city := first(in.City, "Санкт-Петербург")
	slugBase := slugify(in.CompanyName)
	if slugBase == "" {
		slugBase = slugify(in.Name)
	}
	if slugBase == "" {
		slugBase = "fleet"
	}
	slug := slugBase + "-" + strconv.FormatInt(id, 10)
	_, _ = a.db.Exec(r.Context(), `INSERT INTO fleet_profiles(owner_id,slug,title,description,city,published,rating) VALUES($1,$2,$3,'', $4, true, 5.00) ON CONFLICT(owner_id) DO UPDATE SET city=EXCLUDED.city,published=true`, id, slug, first(in.CompanyName, in.Name), city)

	token, _ := a.token(id, "owner")
	write(w, 201, map[string]any{"token": token, "user": User{ID: id, Name: in.Name, Email: in.Email, Phone: in.Phone, Role: "owner", City: city, CompanyName: in.CompanyName}})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Identifier, Password string }
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	idf := strings.TrimSpace(in.Identifier)
	phone := normalizePhone(idf)
	var u User
	var hash string
	err := a.db.QueryRow(r.Context(), `
		SELECT id,name,COALESCE(email,''),COALESCE(phone,''),phone_verified,role,city,company_name,password_hash
		FROM users WHERE email=lower($1) OR phone=$2`, idf, phone).
		Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.PhoneVerified, &u.Role, &u.City, &u.CompanyName, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		write(w, 401, map[string]string{"error": "неверный телефон/email или пароль"})
		return
	}
	t, _ := a.token(u.ID, u.Role)
	write(w, 200, map[string]any{"token": t, "user": u})
}

func hashCode(code string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(code)))
}

func (a *App) requestCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
	}
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	phone := normalizePhone(strings.TrimSpace(in.Phone))
	if len(phone) < 12 {
		write(w, 422, map[string]string{"error": "invalid phone"})
		return
	}
	code := strconv.Itoa(100000 + time.Now().Second()*791%900000)
	_, err := a.db.Exec(r.Context(), `INSERT INTO auth_codes(phone,code_hash,expires_at) VALUES($1,$2,now()+interval '5 minutes')`, phone, hashCode(code))
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// MVP: Telegram bot integration point. In development return the code.
	write(w, 200, map[string]any{"message": "code sent via Telegram", "dev_code": code})
}

func (a *App) verifyCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
		Name  string `json:"name"`
	}
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	phone := normalizePhone(strings.TrimSpace(in.Phone))
	var id int64
	var role string
	err := a.db.QueryRow(r.Context(), `SELECT id,role FROM users WHERE phone=$1`, phone).Scan(&id, &role)
	if err != nil {
		err = a.db.QueryRow(r.Context(), `INSERT INTO users(name,phone,phone_verified,password_hash,role,city,company_name) VALUES($1,$2,true,'','owner','Санкт-Петербург','') RETURNING id,role`, first(in.Name, "KEY user"), phone).Scan(&id, &role)
	} else {
		_, _ = a.db.Exec(r.Context(), `UPDATE users SET phone_verified=true WHERE id=$1`, id)
	}
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	_, err = a.db.Exec(r.Context(), `UPDATE auth_codes SET used_at=now() WHERE phone=$1 AND code_hash=$2 AND used_at IS NULL AND expires_at>now()`, phone, hashCode(in.Code))
	if err != nil {
		write(w, 401, map[string]string{"error": "invalid code"})
		return
	}
	t, _ := a.token(id, role)
	write(w, 200, map[string]any{"token": t, "user": map[string]any{"id": id, "phone": phone, "role": role}})
}

func first(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func slugify(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	var b strings.Builder
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == '-' || r == '_' || r == ' ' {
			b.WriteRune('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

func (a *App) token(id int64, role string) (string, error) {
	c := jwt.MapClaims{"sub": strconv.FormatInt(id, 10), "role": role, "exp": time.Now().Add(24 * time.Hour).Unix()}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(a.jwtSecret)
}

type ctxKey string

func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			write(w, 401, map[string]string{"error": "authorization required"})
			return
		}
		t, e := jwt.Parse(h[7:], func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != "HS256" {
				return nil, errors.New("alg")
			}
			return a.jwtSecret, nil
		})
		if e != nil || !t.Valid {
			write(w, 401, map[string]string{"error": "invalid token"})
			return
		}
		claims, ok := t.Claims.(jwt.MapClaims)
		if !ok {
			write(w, 401, map[string]string{"error": "invalid claims"})
			return
		}
		sub, _ := claims["sub"].(string)
		role, _ := claims["role"].(string)
		ctx := context.WithValue(r.Context(), ctxKey("user"), sub)
		ctx = context.WithValue(ctx, ctxKey("role"), role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (a *App) ownerOnly(next http.Handler) http.Handler {
	return a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(ctxKey("role")) != "owner" {
			write(w, http.StatusForbidden, map[string]string{"error": "owner access required"})
			return
		}
		next.ServeHTTP(w, r)
	}))
}
func userID(ctx context.Context) int64 {
	v, _ := ctx.Value(ctxKey("user")).(string)
	id, _ := strconv.ParseInt(v, 10, 64)
	return id
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	var u User
	err := a.db.QueryRow(r.Context(), `SELECT id,name,COALESCE(email,''),COALESCE(phone,''),phone_verified,role,city,company_name FROM users WHERE id=$1`, userID(r.Context())).
		Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.PhoneVerified, &u.Role, &u.City, &u.CompanyName)
	if err != nil {
		write(w, 404, map[string]string{"error": "user not found"})
		return
	}
	write(w, 200, u)
}

func (a *App) profile(w http.ResponseWriter, r *http.Request) {
	id := userID(r.Context())
	if r.Method == "PATCH" {
		var in struct {
			Name        string `json:"name"`
			Email       string `json:"email"`
			City        string `json:"city"`
			CompanyName string `json:"company_name"`
		}
		if decode(r, &in) != nil {
			write(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		in.Email = strings.ToLower(strings.TrimSpace(in.Email))
		in.City = strings.TrimSpace(in.City)
		in.CompanyName = strings.TrimSpace(in.CompanyName)
		if in.Email != "" && !validEmail(in.Email) {
			write(w, 422, map[string]string{"error": "некорректный email"})
			return
		}
		if in.Name == "" {
			write(w, 422, map[string]string{"error": "имя обязательно"})
			return
		}
		city := first(in.City, "Санкт-Петербург")
		_, err := a.db.Exec(r.Context(), `UPDATE users SET name=$1,email=NULLIF($2,''),city=$3,company_name=$4,updated_at=now() WHERE id=$5`,
			in.Name, in.Email, city, in.CompanyName, id)
		if err != nil {
			write(w, 409, map[string]string{"error": "email уже используется"})
			return
		}
		// Keep the owner's profile and Marketplace storefront in sync.
		title := first(in.CompanyName, in.Name)
		_, err = a.db.Exec(r.Context(), `
			INSERT INTO fleet_profiles(owner_id,slug,title,description,city,published,rating,updated_at)
			VALUES($1,$2,$3,'',$4,true,5.00,now())
			ON CONFLICT(owner_id) DO UPDATE SET title=EXCLUDED.title,city=EXCLUDED.city,updated_at=now()`,
			id, "fleet-"+strconv.FormatInt(id, 10), title, city)
		if err != nil {
			write(w, 500, map[string]string{"error": "не удалось синхронизировать автопарк"})
			return
		}
	}
	a.me(w, r)
}

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	id := userID(r.Context())
	var fleet, available, rented, maintenance int
	var revenue float64
	_ = a.db.QueryRow(r.Context(), `SELECT count(*),count(*) FILTER(WHERE status='available'),count(*) FILTER(WHERE status='rented'),count(*) FILTER(WHERE status='maintenance'),COALESCE(sum(revenue),0) FROM cars WHERE owner_id=$1`, id).
		Scan(&fleet, &available, &rented, &maintenance, &revenue)
	var applications, verificationQueue int
	_ = a.db.QueryRow(r.Context(), `SELECT count(*) FROM rentals WHERE owner_id=$1 AND status IN ('pending','review')`, id).Scan(&applications)
	_ = a.db.QueryRow(r.Context(), `SELECT count(*) FROM verifications WHERE owner_id=$1 AND status IN ('review','pending')`, id).Scan(&verificationQueue)
	util := 0
	if fleet > 0 {
		util = rented * 100 / fleet
	}
	var monthRevenue float64
	_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(sum(amount),0) FROM rentals WHERE owner_id=$1 AND status IN ('active','completed') AND created_at >= date_trunc('month',now())`, id).Scan(&monthRevenue)
	write(w, 200, map[string]any{
		"fleet": fleet, "available": available, "rented": rented, "maintenance": maintenance,
		"revenue": revenue, "monthRevenue": monthRevenue, "utilization": util,
		"applications": applications, "verificationQueue": verificationQueue,
	})
}

func ensureCarPhotosSchema(ctx context.Context, db *pgxpool.Pool) error {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.car_photos') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	path := filepath.Join("/app", "migrations", "007_car_photos.sql")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, string(b))
	return err
}

func (a *App) cars(w http.ResponseWriter, r *http.Request) {
	id := userID(r.Context())
	switch r.Method {
	case "GET":
		rows, err := a.db.Query(r.Context(), `SELECT c.id,c.brand,c.model,c.plate,c.year,c.status,c.revenue,c.location,c.daily_price,c.mileage,c.color,c.vin,c.public_enabled,c.category,c.seats,c.transmission,c.fuel,c.description,c.image_url,c.deposit,c.engine_volume,c.horsepower,c.drive_type,c.fuel_consumption,c.tank_volume,c.maintenance_interval,
COALESCE((SELECT sum(e.amount) FROM car_expenses e WHERE e.car_id=c.id),0),
COALESCE((SELECT sum(COALESCE(r.final_total,r.amount,0)) FROM rentals r WHERE (r.car_id=c.id OR (r.car_id IS NULL AND lower(trim(r.car_name))=lower(trim(c.brand||' '||c.model)))) AND r.status NOT IN ('cancelled','rejected','expired')),0)
FROM cars c WHERE c.owner_id=$1 ORDER BY c.id DESC`, id)
		if err != nil {
			write(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		out := []Car{}
		for rows.Next() {
			var c Car
			if err := rows.Scan(&c.ID, &c.Brand, &c.Model, &c.Plate, &c.Year, &c.Status, &c.Revenue, &c.Location, &c.DailyPrice, &c.Mileage, &c.Color, &c.VIN, &c.PublicEnabled, &c.Category, &c.Seats, &c.Transmission, &c.Fuel, &c.Description, &c.ImageURL, &c.Deposit, &c.EngineVolume, &c.Horsepower, &c.DriveType, &c.FuelConsumption, &c.TankVolume, &c.MaintenanceInterval, &c.Expenses, &c.Earnings); err == nil {
				out = append(out, c)
			}
		}
		write(w, 200, out)
	case "POST":
		var c Car
		if decode(r, &c) != nil {
			write(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		if strings.TrimSpace(c.Brand) == "" || strings.TrimSpace(c.Model) == "" || strings.TrimSpace(c.Plate) == "" || c.Year < 1990 {
			write(w, 422, map[string]string{"error": "заполните марку, модель, госномер и год"})
			return
		}
		err := a.db.QueryRow(r.Context(), `INSERT INTO cars(owner_id,brand,model,plate,year,status,location,daily_price,mileage,color,vin,public_enabled,category,seats,transmission,fuel,description,image_url,deposit,engine_volume,horsepower,drive_type,fuel_consumption,tank_volume,maintenance_interval) VALUES($1,$2,$3,$4,$5,'available',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24) RETURNING id`,
			id, strings.TrimSpace(c.Brand), strings.TrimSpace(c.Model), strings.ToUpper(strings.TrimSpace(c.Plate)), c.Year, first(c.Location, "Санкт-Петербург"), c.DailyPrice, c.Mileage, strings.TrimSpace(c.Color), strings.TrimSpace(c.VIN), c.PublicEnabled, first(c.Category, "Седан"), maxInt(c.Seats, 5), first(c.Transmission, "Автомат"), first(c.Fuel, "Бензин"), strings.TrimSpace(c.Description), strings.TrimSpace(c.ImageURL), c.Deposit, strings.TrimSpace(c.EngineVolume), c.Horsepower, strings.TrimSpace(c.DriveType), strings.TrimSpace(c.FuelConsumption), strings.TrimSpace(c.TankVolume), c.MaintenanceInterval).Scan(&c.ID)
		if err != nil {
			write(w, 409, map[string]string{"error": "не удалось добавить автомобиль"})
			return
		}
		c.Status = "available"
		write(w, 201, c)
	default:
		write(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (a *App) carPhotos(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/car-photos/"), 10, 64)
	if err != nil || id <= 0 {
		write(w, 400, map[string]string{"error": "bad car id"})
		return
	}
	owner := userID(r.Context())
	var owns bool
	if err := a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM cars WHERE id=$1 AND owner_id=$2)`, id, owner).Scan(&owns); err != nil || !owns {
		write(w, 404, map[string]string{"error": "автомобиль не найден"})
		return
	}
	switch r.Method {
	case "GET":
		rows, err := a.db.Query(r.Context(), `SELECT id,url,filename,created_at FROM car_photos WHERE car_id=$1 ORDER BY is_primary DESC,id ASC`, id)
		if err != nil {
			write(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var pid int64
			var url, filename string
			var created time.Time
			if err := rows.Scan(&pid, &url, &filename, &created); err == nil {
				out = append(out, map[string]any{"id": pid, "url": url, "filename": filename, "created_at": created})
			}
		}
		write(w, 200, out)
	case "POST":
		r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
		if err := r.ParseMultipartForm(12 << 20); err != nil {
			write(w, 413, map[string]string{"error": "файл слишком большой"})
			return
		}
		file, header, err := r.FormFile("photo")
		if err != nil {
			write(w, 400, map[string]string{"error": "выберите фотографию"})
			return
		}
		defer file.Close()
		if header.Size > 10<<20 {
			write(w, 413, map[string]string{"error": "максимальный размер фото — 10 МБ"})
			return
		}
		contentType := header.Header.Get("Content-Type")
		allowed := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}
		ext, ok := allowed[contentType]
		if !ok {
			write(w, 415, map[string]string{"error": "поддерживаются JPG, PNG и WebP"})
			return
		}
		var buf [16]byte
		if _, err := file.Read(buf[:]); err != nil {
			write(w, 400, map[string]string{"error": "не удалось прочитать файл"})
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			write(w, 400, map[string]string{"error": "не удалось обработать файл"})
			return
		}
		var rb [12]byte
		if _, err := rand.Read(rb[:]); err != nil {
			write(w, 500, map[string]string{"error": "не удалось создать имя файла"})
			return
		}
		name := hex.EncodeToString(rb[:]) + ext
		path := filepath.Join("/app/uploads/cars", name)
		dst, err := os.Create(path)
		if err != nil {
			write(w, 500, map[string]string{"error": "не удалось сохранить фото"})
			return
		}
		if _, err = file.Seek(0, 0); err == nil {
			_, err = io.Copy(dst, file)
		}
		dst.Close()
		if err != nil {
			_ = os.Remove(path)
			write(w, 500, map[string]string{"error": "не удалось сохранить фото"})
			return
		}
		url := "/uploads/cars/" + name
		var pid int64
		var count int
		_ = a.db.QueryRow(r.Context(), `SELECT count(*) FROM car_photos WHERE car_id=$1`, id).Scan(&count)
		if err := a.db.QueryRow(r.Context(), `INSERT INTO car_photos(car_id,url,filename,is_primary) VALUES($1,$2,$3,$4) RETURNING id`, id, url, header.Filename, count == 0).Scan(&pid); err != nil {
			_ = os.Remove(path)
			write(w, 500, map[string]string{"error": "не удалось сохранить запись фото"})
			return
		}
		if count == 0 {
			_, _ = a.db.Exec(r.Context(), `UPDATE cars SET image_url=$1 WHERE id=$2 AND owner_id=$3`, url, id, owner)
		}
		write(w, 201, map[string]any{"id": pid, "url": url, "filename": header.Filename})
	case "DELETE":
		photoID, err := strconv.ParseInt(r.URL.Query().Get("photo_id"), 10, 64)
		if err != nil || photoID <= 0 {
			write(w, 400, map[string]string{"error": "bad photo id"})
			return
		}
		var url string
		if err := a.db.QueryRow(r.Context(), `DELETE FROM car_photos WHERE id=$1 AND car_id=$2 RETURNING url`, photoID, id).Scan(&url); err != nil {
			write(w, 404, map[string]string{"error": "фото не найдено"})
			return
		}
		_ = os.Remove(filepath.Join("/app", strings.TrimPrefix(url, "/")))
		var primary string
		_ = a.db.QueryRow(r.Context(), `SELECT url FROM car_photos WHERE car_id=$1 ORDER BY id ASC LIMIT 1`, id).Scan(&primary)
		_, _ = a.db.Exec(r.Context(), `UPDATE car_photos SET is_primary=false WHERE car_id=$1`, id)
		if primary != "" {
			_, _ = a.db.Exec(r.Context(), `UPDATE car_photos SET is_primary=true WHERE car_id=$1 AND url=$2`, id, primary)
		}
		_, _ = a.db.Exec(r.Context(), `UPDATE cars SET image_url=COALESCE((SELECT url FROM car_photos WHERE car_id=$1 AND is_primary LIMIT 1),'') WHERE id=$1 AND owner_id=$2`, id, owner)
		write(w, 200, map[string]bool{"ok": true})
	default:
		write(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (a *App) carFinance(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/car-finance/"), 10, 64)
	if err != nil || id <= 0 {
		write(w, 400, map[string]string{"error": "bad car id"})
		return
	}
	owner := userID(r.Context())
	var owns bool
	if err := a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM cars WHERE id=$1 AND owner_id=$2)`, id, owner).Scan(&owns); err != nil || !owns {
		write(w, 404, map[string]string{"error": "автомобиль не найден"})
		return
	}
	if r.Method == "GET" {
		var earnings, expenses float64
		_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(sum(COALESCE(final_total,amount,0)),0) FROM rentals WHERE (car_id=$1 OR (car_id IS NULL AND lower(trim(car_name))=lower(trim((SELECT brand||' '||model FROM cars WHERE id=$1))))) AND status NOT IN ('cancelled','rejected','expired')`, id).Scan(&earnings)
		_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(sum(amount),0) FROM car_expenses WHERE car_id=$1`, id).Scan(&expenses)
		rows, _ := a.db.Query(r.Context(), `SELECT id,amount,expense_type,note,created_at FROM car_expenses WHERE car_id=$1 ORDER BY created_at DESC,id DESC`, id)
		expensesList := []map[string]any{}
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var x int64
				var amount float64
				var category, note string
				var at time.Time
				if rows.Scan(&x, &amount, &category, &note, &at) == nil {
					expensesList = append(expensesList, map[string]any{"id": x, "amount": amount, "category": category, "note": note, "created_at": at})
				}
			}
		}
		dealsRows, _ := a.db.Query(r.Context(), `SELECT id,COALESCE(booking_code,''),client_name,COALESCE(final_total,amount),starts_at FROM rentals WHERE (car_id=$1 OR (car_id IS NULL AND lower(trim(car_name))=lower(trim((SELECT brand||' '||model FROM cars WHERE id=$1))))) AND status NOT IN ('cancelled','rejected','expired') ORDER BY starts_at DESC NULLS LAST,id DESC LIMIT 30`, id)
		deals := []map[string]any{}
		if dealsRows != nil {
			defer dealsRows.Close()
			for dealsRows.Next() {
				var x int64
				var code, client string
				var amount float64
				var at *time.Time
				if dealsRows.Scan(&x, &code, &client, &amount, &at) == nil {
					deals = append(deals, map[string]any{"id": x, "code": code, "client": client, "amount": amount, "starts_at": at})
				}
			}
		}
		write(w, 200, map[string]any{"earnings": earnings, "expenses_total": expenses, "net": earnings - expenses, "expenses": expensesList, "deals": deals})
		return
	}
	if r.Method == "POST" {
		var in struct {
			Amount   float64 `json:"amount"`
			Category string  `json:"category"`
			Note     string  `json:"note"`
		}
		if decode(r, &in) != nil || in.Amount <= 0 {
			write(w, 422, map[string]string{"error": "укажите положительную сумму расхода"})
			return
		}
		_, err = a.db.Exec(r.Context(), `INSERT INTO car_expenses(owner_id,car_id,amount,expense_type,note) VALUES($1,$2,$3,$4,$5)`, owner, id, in.Amount, strings.TrimSpace(in.Category), strings.TrimSpace(in.Note))
		if err != nil {
			write(w, 500, map[string]string{"error": "не удалось сохранить расход"})
			return
		}
		write(w, 201, map[string]bool{"ok": true})
		return
	}
	if r.Method == "DELETE" {
		expenseID, e := strconv.ParseInt(r.URL.Query().Get("expense_id"), 10, 64)
		if e != nil || expenseID <= 0 {
			write(w, 400, map[string]string{"error": "bad expense id"})
			return
		}
		if _, e = a.db.Exec(r.Context(), `DELETE FROM car_expenses WHERE id=$1 AND car_id=$2`, expenseID, id); e != nil {
			write(w, 500, map[string]string{"error": "не удалось удалить расход"})
			return
		}
		write(w, 200, map[string]bool{"ok": true})
		return
	}
	write(w, 405, map[string]string{"error": "method not allowed"})
}

// func (a *App) carFinance(w http.ResponseWriter, r *http.Request) {
// 	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/car-finance/"), 10, 64)
// 	if err != nil || id <= 0 {
// 		write(w, 400, map[string]string{"error": "bad car id"})
// 		return
// 	}
// 	owner := userID(r.Context())
// 	var brand, model, plate string
// 	if err := a.db.QueryRow(r.Context(), `SELECT brand,model,plate FROM cars WHERE id=$1 AND owner_id=$2`, id, owner).Scan(&brand, &model, &plate); err != nil {
// 		write(w, 404, map[string]string{"error": "автомобиль не найден"})
// 		return
// 	}
// 	switch r.Method {
// 	case "GET":
// 		var earned, expenses float64
// 		_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(CASE WHEN status NOT IN ('cancelled','rejected','expired') THEN COALESCE(final_total,amount) ELSE 0 END),0), COALESCE(SUM(CASE WHEN status NOT IN ('cancelled','rejected','expired') THEN 1 ELSE 0 END),0) FROM rentals WHERE owner_id=$1 AND car_id=$2`, owner, id).Scan(&earned, new(float64))
// 		_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(amount),0) FROM car_expenses WHERE owner_id=$1 AND car_id=$2`, owner, id).Scan(&expenses)
// 		rows, e := a.db.Query(r.Context(), `SELECT id,COALESCE(final_total,amount),status,starts_at,ends_at FROM rentals WHERE owner_id=$1 AND car_id=$2 AND status NOT IN ('cancelled','rejected','expired') ORDER BY starts_at DESC NULLS LAST,id DESC`, owner, id)
// 		if e != nil {
// 			write(w, 500, map[string]string{"error": e.Error()})
// 			return
// 		}
// 		defer rows.Close()
// 		deals := []map[string]any{}
// 		for rows.Next() {
// 			var rid int64
// 			var amount float64
// 			var status string
// 			var starts, ends *time.Time
// 			if rows.Scan(&rid, &amount, &status, &starts, &ends) == nil {
// 				deals = append(deals, map[string]any{"id": rid, "amount": amount, "status": status, "starts_at": starts, "ends_at": ends})
// 			}
// 		}
// 		expRows, e := a.db.Query(r.Context(), `SELECT id,expense_type,amount,note,created_at FROM car_expenses WHERE owner_id=$1 AND car_id=$2 ORDER BY created_at DESC,id DESC`, owner, id)
// 		if e != nil {
// 			write(w, 500, map[string]string{"error": e.Error()})
// 			return
// 		}
// 		defer expRows.Close()
// 		exps := []map[string]any{}
// 		for expRows.Next() {
// 			var eid int64
// 			var kind, note string
// 			var amount float64
// 			var at time.Time
// 			if expRows.Scan(&eid, &kind, &amount, &note, &at) == nil {
// 				exps = append(exps, map[string]any{"id": eid, "kind": kind, "amount": amount, "note": note, "created_at": at})
// 			}
// 		}
// 		write(w, 200, map[string]any{"car": map[string]any{"id": id, "brand": brand, "model": model, "plate": plate}, "earned": earned, "expenses": expenses, "profit": earned - expenses, "deals": deals, "expenses_list": exps})
// 	case "POST":
// 		var in struct {
// 			Kind   string  `json:"kind"`
// 			Amount float64 `json:"amount"`
// 			Note   string  `json:"note"`
// 		}
// 		if decode(r, &in) != nil || in.Amount <= 0 {
// 			write(w, 422, map[string]string{"error": "укажите сумму расхода"})
// 			return
// 		}
// 		kind := first(strings.TrimSpace(in.Kind), "Другое")
// 		var eid int64
// 		if err := a.db.QueryRow(r.Context(), `INSERT INTO car_expenses(owner_id,car_id,expense_type,amount,note) VALUES($1,$2,$3,$4,$5) RETURNING id`, owner, id, kind, in.Amount, strings.TrimSpace(in.Note)).Scan(&eid); err != nil {
// 			write(w, 500, map[string]string{"error": "не удалось сохранить расход"})
// 			return
// 		}
// 		write(w, 201, map[string]any{"id": eid, "ok": true})
// 	case "DELETE":
// 		expID, e := strconv.ParseInt(r.URL.Query().Get("expense_id"), 10, 64)
// 		if e != nil || expID <= 0 {
// 			write(w, 400, map[string]string{"error": "bad expense id"})
// 			return
// 		}
// 		tag, e := a.db.Exec(r.Context(), `DELETE FROM car_expenses WHERE id=$1 AND car_id=$2 AND owner_id=$3`, expID, id, owner)
// 		if e != nil || tag.RowsAffected() == 0 {
// 			write(w, 404, map[string]string{"error": "расход не найден"})
// 			return
// 		}
// 		write(w, 200, map[string]bool{"ok": true})
// 	default:
// 		write(w, 405, map[string]string{"error": "method not allowed"})
// 	}
// }

func (a *App) carByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/cars/"), 10, 64)
	if err != nil {
		write(w, 400, map[string]string{"error": "bad id"})
		return
	}
	owner := userID(r.Context())
	if r.Method == "PATCH" {
		var c Car
		if decode(r, &c) != nil {
			write(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		_, err := a.db.Exec(r.Context(), `UPDATE cars SET status=COALESCE(NULLIF($1,''),status),daily_price=CASE WHEN $2>=0 THEN $2 ELSE daily_price END,location=COALESCE(NULLIF($3,''),location),mileage=CASE WHEN $4>=0 THEN $4 ELSE mileage END,public_enabled=$5,category=COALESCE(NULLIF($6,''),category),seats=CASE WHEN $7>0 THEN $7 ELSE seats END,transmission=COALESCE(NULLIF($8,''),transmission),fuel=COALESCE(NULLIF($9,''),fuel),description=COALESCE(NULLIF($10,''),description),image_url=COALESCE(NULLIF($11,''),image_url),deposit=CASE WHEN $12>0 THEN $12 ELSE deposit END WHERE id=$13 AND owner_id=$14`,
			c.Status, c.DailyPrice, c.Location, c.Mileage, c.PublicEnabled, c.Category, c.Seats, c.Transmission, c.Fuel, c.Description, c.ImageURL, c.Deposit, id, owner)
		if err != nil {
			write(w, 500, map[string]string{"error": "не удалось обновить автомобиль"})
			return
		}
		write(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.Method == "DELETE" {
		tag, err := a.db.Exec(r.Context(), `DELETE FROM cars WHERE id=$1 AND owner_id=$2`, id, owner)
		if err != nil || tag.RowsAffected() == 0 {
			write(w, 404, map[string]string{"error": "автомобиль не найден"})
			return
		}
		write(w, 200, map[string]bool{"ok": true})
		return
	}
	write(w, 405, map[string]string{"error": "method not allowed"})
}

func (a *App) rentals(w http.ResponseWriter, r *http.Request) {
	owner := userID(r.Context())
	if r.Method == "POST" {
		var in struct {
			CarName     string  `json:"car_name"`
			ClientName  string  `json:"client_name"`
			ClientPhone string  `json:"client_phone"`
			Status      string  `json:"status"`
			Amount      float64 `json:"amount"`
			StartsAt    string  `json:"starts_at"`
			EndsAt      string  `json:"ends_at"`
		}
		if decode(r, &in) != nil {
			write(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		status := first(in.Status, "pending")
		var id int64
		err := a.db.QueryRow(r.Context(), `INSERT INTO rentals(owner_id,car_name,client_name,client_phone,status,amount,final_total,starts_at,ends_at) VALUES($1,$2,$3,$4,$5,$6,$6,NULLIF($7,'')::timestamptz,NULLIF($8,'')::timestamptz) RETURNING id`,
			owner, in.CarName, in.ClientName, normalizePhone(in.ClientPhone), status, in.Amount, in.StartsAt, in.EndsAt).Scan(&id)
		if err != nil {
			write(w, 500, map[string]string{"error": "не удалось создать аренду"})
			return
		}
		_, _ = a.db.Exec(r.Context(), `INSERT INTO rental_events(rental_id,actor_id,actor_role,event_type,to_status,payload) VALUES($1,$2,'owner','manual_created',$3,$4::jsonb)`, id, owner, status, mustJSON(map[string]any{"source": "owner"}))
		write(w, 201, map[string]any{"id": id, "ok": true})
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT id,COALESCE(booking_code,''),car_name,client_name,client_phone,status,amount,deposit,starts_at,ends_at,payment_status,COALESCE(final_total,amount) FROM rentals WHERE owner_id=$1 ORDER BY starts_at DESC NULLS LAST,id DESC`, owner)
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var code, car, client, phone, status, payment string
		var amount, deposit, finalTotal float64
		var starts, ends *time.Time
		if rows.Scan(&id, &code, &car, &client, &phone, &status, &amount, &deposit, &starts, &ends, &payment, &finalTotal) == nil {
			out = append(out, map[string]any{"id": id, "booking_code": code, "car": car, "client": client, "phone": phone, "status": status, "amount": amount, "deposit": deposit, "starts_at": starts, "ends_at": ends, "payment_status": payment, "final_total": finalTotal})
		}
	}
	write(w, 200, out)
}

func (a *App) rentalByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/rentals/"), 10, 64)
	if err != nil {
		write(w, 400, map[string]string{"error": "bad id"})
		return
	}
	if r.Method != "PATCH" {
		write(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var in struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if decode(r, &in) != nil || !contains([]string{"hold", "pending", "review", "confirmed", "preparing", "active", "returned", "completed", "cancelled", "expired", "rejected"}, in.Status) {
		write(w, 422, map[string]string{"error": "invalid status"})
		return
	}
	if err := a.transitionRental(r.Context(), id, userID(r.Context()), "owner", in.Status, in.Reason); err != nil {
		code := 409
		if errors.Is(err, errRentalNotFound) {
			code = 404
		}
		write(w, code, map[string]string{"error": err.Error()})
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

var errRentalNotFound = errors.New("аренда не найдена")

func allowedRentalTransition(from, to string) bool {
	if from == to {
		return true
	}
	m := map[string][]string{
		"hold":      {"confirmed", "expired", "cancelled", "rejected"},
		"pending":   {"review", "confirmed", "rejected", "cancelled"},
		"review":    {"confirmed", "rejected", "cancelled"},
		"confirmed": {"preparing", "active", "cancelled", "rejected"},
		"preparing": {"active", "cancelled"},
		"active":    {"returned", "cancelled"},
		"returned":  {"completed"},
	}
	return contains(m[from], to)
}

func (a *App) transitionRental(ctx context.Context, rentalID, actorID int64, actorRole, to, reason string) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var ownerID int64
	var from string
	err = tx.QueryRow(ctx, `SELECT owner_id,status FROM rentals WHERE id=$1 FOR UPDATE`, rentalID).Scan(&ownerID, &from)
	if err != nil {
		return errRentalNotFound
	}
	if actorRole == "owner" && ownerID != actorID {
		return errRentalNotFound
	}
	if !allowedRentalTransition(from, to) {
		return fmt.Errorf("нельзя перевести аренду из «%s» в «%s»", from, to)
	}
	_, err = tx.Exec(ctx, `UPDATE rentals SET status=$1, cancellation_reason=CASE WHEN $1 IN ('cancelled','rejected') THEN $2 ELSE cancellation_reason END, pickup_at=CASE WHEN $1='active' AND pickup_at IS NULL THEN now() ELSE pickup_at END, returned_at=CASE WHEN $1='returned' THEN now() ELSE returned_at END, updated_at=now() WHERE id=$3`, to, strings.TrimSpace(reason), rentalID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO rental_events(rental_id,actor_id,actor_role,event_type,from_status,to_status,payload) VALUES($1,$2,$3,'status_change',$4,$5,$6::jsonb)`, rentalID, actorID, actorRole, from, to, mustJSON(map[string]any{"reason": strings.TrimSpace(reason)}))
	if err != nil {
		return err
	}
	if to == "completed" {
		_, _ = tx.Exec(ctx, `UPDATE cars c SET status='available', revenue=c.revenue+r.final_total FROM rentals r WHERE r.id=$1 AND c.id=r.car_id`, rentalID)
	} else if to == "active" {
		_, _ = tx.Exec(ctx, `UPDATE cars SET status='rented' WHERE id=(SELECT car_id FROM rentals WHERE id=$1)`, rentalID)
	} else if to == "returned" || to == "cancelled" || to == "rejected" || to == "expired" {
		_, _ = tx.Exec(ctx, `UPDATE cars SET status='available' WHERE id=(SELECT car_id FROM rentals WHERE id=$1) AND status='rented'`, rentalID)
	}
	return tx.Commit(ctx)
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func (a *App) rentalCalendar(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		from = time.Now().Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	}
	st, en, err := parseBookingTimes(from+"T00:00:00Z", to+"T00:00:00Z")
	if err != nil {
		write(w, 422, map[string]string{"error": "некорректный диапазон"})
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT c.id,c.brand,c.model,c.plate,c.status,COALESCE(c.daily_price,0),r.id,COALESCE(r.booking_code,''),COALESCE(r.client_name,''),r.status,r.starts_at,r.ends_at,COALESCE(r.amount,0) FROM cars c LEFT JOIN rentals r ON r.car_id=c.id AND r.owner_id=$1 AND r.starts_at < $3 AND r.ends_at > $2 AND r.status NOT IN ('cancelled','rejected','expired') WHERE c.owner_id=$1 ORDER BY c.id,r.starts_at`, userID(r.Context()), st, en)
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	by := map[int64]map[string]any{}
	order := []int64{}
	for rows.Next() {
		var cid int64
		var brand, model, plate, cstatus string
		var price float64
		var rid *int64
		var code, client, rstatus string
		var rs, re *time.Time
		var amount float64
		if rows.Scan(&cid, &brand, &model, &plate, &cstatus, &price, &rid, &code, &client, &rstatus, &rs, &re, &amount) != nil {
			continue
		}
		if _, ok := by[cid]; !ok {
			by[cid] = map[string]any{"id": cid, "car": brand + " " + model, "plate": plate, "status": cstatus, "daily_price": price, "bookings": []any{}}
			order = append(order, cid)
		}
		if rid != nil {
			b := by[cid]["bookings"].([]any)
			b = append(b, map[string]any{"id": *rid, "code": code, "client": client, "status": rstatus, "starts_at": rs, "ends_at": re, "amount": amount})
			by[cid]["bookings"] = b
		}
	}
	out := []map[string]any{}
	for _, id := range order {
		out = append(out, by[id])
	}
	write(w, 200, map[string]any{"from": st, "to": en, "cars": out})
}

func (a *App) rentalMeeting(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/rentals/meeting/"), 10, 64)
	if err != nil {
		write(w, 400, map[string]string{"error": "bad id"})
		return
	}
	if r.Method != "PATCH" {
		write(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var in struct {
		Kind     string `json:"kind"`
		At       string `json:"at"`
		Location string `json:"location"`
	}
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	kind := first(in.Kind, "pickup")
	if kind != "pickup" && kind != "return" {
		write(w, 422, map[string]string{"error": "kind must be pickup or return"})
		return
	}
	at, err := time.Parse(time.RFC3339, in.At)
	if err != nil {
		at, err = time.Parse("2006-01-02T15:04", in.At)
	}
	if err != nil {
		write(w, 422, map[string]string{"error": "некорректная дата встречи"})
		return
	}
	colAt := "pickup_meeting_at"
	colLoc := "pickup_meeting_location"
	if kind == "return" {
		colAt = "return_meeting_at"
		colLoc = "return_meeting_location"
	}
	q := fmt.Sprintf(`UPDATE rentals SET %s=$1,%s=$2,updated_at=now() WHERE id=$3 AND owner_id=$4`, colAt, colLoc)
	if _, err = a.db.Exec(r.Context(), q, at, strings.TrimSpace(in.Location), id, userID(r.Context())); err != nil {
		write(w, 500, map[string]string{"error": "не удалось назначить встречу"})
		return
	}
	_, _ = a.db.Exec(r.Context(), `INSERT INTO rental_events(rental_id,actor_id,actor_role,event_type,payload) VALUES($1,$2,'owner','meeting_scheduled',$3::jsonb)`, id, userID(r.Context()), mustJSON(map[string]any{"kind": kind, "at": at, "location": strings.TrimSpace(in.Location)}))
	write(w, 200, map[string]bool{"ok": true})
}

func (a *App) rentalPayment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/rentals/payment/"), 10, 64)
	if err != nil {
		write(w, 400, map[string]string{"error": "bad id"})
		return
	}
	if r.Method != "PATCH" {
		write(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var in struct {
		Paid bool `json:"paid"`
	}
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	status := "unpaid"
	if in.Paid {
		status = "paid"
	}
	if _, err = a.db.Exec(r.Context(), `UPDATE rentals SET payment_status=$1,updated_at=now() WHERE id=$2 AND owner_id=$3`, status, id, userID(r.Context())); err != nil {
		write(w, 500, map[string]string{"error": "не удалось обновить оплату"})
		return
	}
	if in.Paid {
		_, _ = a.db.Exec(r.Context(), `INSERT INTO rental_payments(rental_id,payment_type,status,amount,provider) SELECT id,'rental','paid',final_total,'manual' FROM rentals WHERE id=$1 AND owner_id=$2`, id, userID(r.Context()))
	}
	_, _ = a.db.Exec(r.Context(), `INSERT INTO rental_events(rental_id,actor_id,actor_role,event_type,payload) VALUES($1,$2,'owner','payment_status',$3::jsonb)`, id, userID(r.Context()), mustJSON(map[string]any{"status": status}))
	write(w, 200, map[string]string{"payment_status": status})
}

func (a *App) rentalOps(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/rental-ops/"), 10, 64)
	if err != nil {
		write(w, 400, map[string]string{"error": "bad id"})
		return
	}
	owner := userID(r.Context())
	if r.Method == "GET" {
		var d = map[string]any{}
		var code, car, client, status, payment string
		var amount, deposit, finalTotal, late, damage float64
		var st, en, pickup, returned, pickupMeeting, returnMeeting *time.Time
		var mileageStart, mileageEnd, fuelStart, fuelEnd *int
		var pickupMeetingLocation, returnMeetingLocation string
		err = a.db.QueryRow(r.Context(), `SELECT COALESCE(r.booking_code,''),r.car_name,r.client_name,r.status,r.payment_status,r.amount,r.deposit,r.final_total,r.late_fee,r.damage_fee,r.starts_at,r.ends_at,r.pickup_at,r.returned_at,r.odometer_start,r.odometer_end,r.fuel_start,r.fuel_end,r.pickup_meeting_at,r.pickup_meeting_location,r.return_meeting_at,r.return_meeting_location FROM rentals r WHERE r.id=$1 AND r.owner_id=$2`, id, owner).Scan(&code, &car, &client, &status, &payment, &amount, &deposit, &finalTotal, &late, &damage, &st, &en, &pickup, &returned, &mileageStart, &mileageEnd, &fuelStart, &fuelEnd, &pickupMeeting, &pickupMeetingLocation, &returnMeeting, &returnMeetingLocation)
		if err != nil {
			write(w, 404, map[string]string{"error": "аренда не найдена"})
			return
		}
		d["id"] = id
		d["booking_code"] = code
		d["car"] = car
		d["client"] = client
		d["status"] = status
		d["payment_status"] = payment
		d["amount"] = amount
		d["deposit"] = deposit
		d["final_total"] = finalTotal
		d["late_fee"] = late
		d["damage_fee"] = damage
		d["starts_at"] = st
		d["ends_at"] = en
		d["pickup_at"] = pickup
		d["returned_at"] = returned
		d["pickup_meeting_at"] = pickupMeeting
		d["pickup_meeting_location"] = pickupMeetingLocation
		d["return_meeting_at"] = returnMeeting
		d["return_meeting_location"] = returnMeetingLocation
		d["odometer_start"] = mileageStart
		d["odometer_end"] = mileageEnd
		d["fuel_start"] = fuelStart
		d["fuel_end"] = fuelEnd
		d["events"] = rentalEvents(r.Context(), a.db, id)
		d["extras"] = rentalExtras(r.Context(), a.db, id)
		d["payments"] = rentalPayments(r.Context(), a.db, id)
		d["inspections"] = rentalInspections(r.Context(), a.db, id)
		d["expenses"] = rentalExpenses(r.Context(), a.db, id)
		d["adjustments"] = rentalAdjustments(r.Context(), a.db, id)
		d["deposit_transactions"] = depositTransactions(r.Context(), a.db, id)
		var expensesTotal, adjustmentsTotal float64
		_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(sum(amount),0) FROM rental_expenses WHERE rental_id=$1`, id).Scan(&expensesTotal)
		_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(sum(CASE WHEN adjustment_type IN ('late_fee','damage_fee','other') THEN amount ELSE -amount END),0) FROM rental_adjustments WHERE rental_id=$1`, id).Scan(&adjustmentsTotal)
		d["expenses_total"] = expensesTotal
		d["profit"] = finalTotal - expensesTotal
		d["extension_count"] = rentalExtensionCount(r.Context(), a.db, id)
		write(w, 200, d)
		return
	}
	if r.Method != "POST" {
		write(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var in struct {
		Type        string   `json:"type"`
		Name        string   `json:"name"`
		Qty         int      `json:"qty"`
		UnitPrice   float64  `json:"unit_price"`
		Amount      float64  `json:"amount"`
		Note        string   `json:"note"`
		Kind        string   `json:"kind"`
		Mileage     *int     `json:"mileage"`
		Fuel        *int     `json:"fuel_level"`
		Photos      []string `json:"photos"`
		PaymentType string   `json:"payment_type"`
		NewStart    string   `json:"new_start"`
		NewEnd      string   `json:"new_end"`
	}
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	if in.Type == "extra" {
		if in.Qty < 1 {
			in.Qty = 1
		}
		_, err = a.db.Exec(r.Context(), `INSERT INTO rental_extras(rental_id,name,qty,unit_price,total) SELECT $1,$2,$3,$4,$3*$4 FROM rentals WHERE id=$1 AND owner_id=$5`, id, in.Name, in.Qty, in.UnitPrice, owner)
		if err == nil {
			_, _ = a.db.Exec(r.Context(), `UPDATE rentals SET final_total=GREATEST(amount+COALESCE((SELECT sum(total) FROM rental_extras WHERE rental_id=$1),0)+COALESCE((SELECT sum(CASE WHEN adjustment_type IN ('late_fee','damage_fee','other') THEN amount ELSE -amount END) FROM rental_adjustments WHERE rental_id=$1),0),0),updated_at=now() WHERE id=$1 AND owner_id=$2`, id, owner)
		}
	} else if in.Type == "payment" {
		_, err = a.db.Exec(r.Context(), `INSERT INTO rental_payments(rental_id,payment_type,status,amount,provider) SELECT $1,$2,'paid',$3,'mock' FROM rentals WHERE id=$1 AND owner_id=$4`, id, first(in.PaymentType, "rental"), in.Amount, owner)
		if err == nil {
			_, _ = a.db.Exec(r.Context(), `UPDATE rentals SET payment_status='paid',updated_at=now() WHERE id=$1 AND owner_id=$2`, id, owner)
		}
	} else if in.Type == "inspection" {
		b, _ := json.Marshal(in.Photos)
		_, err = a.db.Exec(r.Context(), `INSERT INTO rental_inspections(rental_id,kind,mileage,fuel_level,notes,photos) SELECT $1,$2,$3,$4,$5,$6::jsonb FROM rentals WHERE id=$1 AND owner_id=$7`, id, first(in.Kind, "pickup"), in.Mileage, in.Fuel, in.Note, string(b), owner)
	} else if in.Type == "expense" {
		_, err = a.db.Exec(r.Context(), `INSERT INTO rental_expenses(rental_id,expense_type,amount,note) SELECT $1,$2,$3,$4 FROM rentals WHERE id=$1 AND owner_id=$5`, id, first(in.Kind, "other"), in.Amount, in.Note, owner)
	} else if in.Type == "adjustment" {
		kind := first(in.Kind, "other")
		if !contains([]string{"late_fee", "damage_fee", "discount", "other"}, kind) || in.Amount == 0 {
			write(w, 422, map[string]string{"error": "укажите тип и сумму корректировки"})
			return
		}
		_, err = a.db.Exec(r.Context(), `INSERT INTO rental_adjustments(rental_id,adjustment_type,amount,note) SELECT $1,$2,$3,$4 FROM rentals WHERE id=$1 AND owner_id=$5`, id, kind, in.Amount, in.Note, owner)
		if err == nil {
			_, _ = a.db.Exec(r.Context(), `UPDATE rentals SET late_fee=COALESCE((SELECT sum(amount) FROM rental_adjustments WHERE rental_id=$1 AND adjustment_type='late_fee'),0), damage_fee=COALESCE((SELECT sum(amount) FROM rental_adjustments WHERE rental_id=$1 AND adjustment_type='damage_fee'),0), final_total=GREATEST(amount+COALESCE((SELECT sum(CASE WHEN adjustment_type IN ('late_fee','damage_fee','other') THEN amount ELSE -amount END) FROM rental_adjustments WHERE rental_id=$1),0)+COALESCE((SELECT sum(total) FROM rental_extras WHERE rental_id=$1),0),0),updated_at=now() WHERE id=$1 AND owner_id=$2`, id, owner)
		}
	} else if in.Type == "deposit" {
		kind := first(in.Kind, "hold")
		if !contains([]string{"hold", "release", "charge"}, kind) || in.Amount <= 0 {
			write(w, 422, map[string]string{"error": "укажите операцию и положительную сумму депозита"})
			return
		}
		_, err = a.db.Exec(r.Context(), `INSERT INTO deposit_transactions(rental_id,transaction_type,amount,note) SELECT $1,$2,$3,$4 FROM rentals WHERE id=$1 AND owner_id=$5`, id, kind, in.Amount, in.Note, owner)
	} else if in.Type == "extension" {
		if in.NewEnd == "" {
			write(w, 422, map[string]string{"error": "укажите новую дату возврата"})
			return
		}
		newEnd, parseErr := time.Parse(time.RFC3339, in.NewEnd)
		if parseErr != nil {
			newEnd, parseErr = time.Parse("2006-01-02T15:04", in.NewEnd)
		}
		if parseErr != nil {
			write(w, 422, map[string]string{"error": "некорректная дата возврата"})
			return
		}
		var currentStart time.Time
		var currentEnd time.Time
		if err = a.db.QueryRow(r.Context(), `SELECT starts_at,ends_at FROM rentals WHERE id=$1 AND owner_id=$2`, id, owner).Scan(&currentStart, &currentEnd); err != nil {
			write(w, 404, map[string]string{"error": "аренда не найдена"})
			return
		}
		if !newEnd.After(currentEnd) {
			write(w, 422, map[string]string{"error": "новая дата должна быть позже текущей"})
			return
		}
		var blocked bool
		if err = a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM rentals r1 JOIN rentals r2 ON r2.car_id=r1.car_id WHERE r1.id=$1 AND r2.id<>r1.id AND r2.status IN ('hold','pending','review','confirmed','preparing','active') AND r2.starts_at < $3 AND r2.ends_at > $2)`, id, currentStart, newEnd).Scan(&blocked); err != nil {
			write(w, 500, map[string]string{"error": "не удалось проверить доступность"})
			return
		}
		if blocked {
			write(w, 409, map[string]string{"error": "автомобиль занят в выбранный период"})
			return
		}
		_, err = a.db.Exec(r.Context(), `UPDATE rentals SET ends_at=$1,extension_count=extension_count+1,updated_at=now() WHERE id=$2 AND owner_id=$3`, newEnd, id, owner)
		if err == nil {
			_, _ = a.db.Exec(r.Context(), `INSERT INTO rental_events(rental_id,actor_id,actor_role,event_type,payload) VALUES($1,$2,'owner','extension',$3::jsonb)`, id, owner, mustJSON(map[string]any{"new_end": newEnd}))
		}
	} else {
		write(w, 422, map[string]string{"error": "unknown operation"})
		return
	}
	if err != nil {
		write(w, 500, map[string]string{"error": "не удалось сохранить операцию"})
		return
	}
	if in.Type == "inspection" && in.Kind == "pickup" {
		_, _ = a.db.Exec(r.Context(), `UPDATE rentals SET odometer_start=COALESCE($1,odometer_start),fuel_start=COALESCE($2,fuel_start),updated_at=now() WHERE id=$3 AND owner_id=$4`, in.Mileage, in.Fuel, id, owner)
	}
	if in.Type == "inspection" && in.Kind == "return" {
		_, _ = a.db.Exec(r.Context(), `UPDATE rentals SET odometer_end=COALESCE($1,odometer_end),fuel_end=COALESCE($2,fuel_end),final_total=GREATEST(amount+COALESCE((SELECT sum(total) FROM rental_extras WHERE rental_id=$3),0)+COALESCE(late_fee,0)+COALESCE(damage_fee,0),0),updated_at=now() WHERE id=$3 AND owner_id=$4`, in.Mileage, in.Fuel, id, owner)
	}
	write(w, 201, map[string]bool{"ok": true})
}

func rentalAdjustments(ctx context.Context, db *pgxpool.Pool, id int64) []any {
	rows, e := db.Query(ctx, `SELECT id,adjustment_type,amount,note,created_at FROM rental_adjustments WHERE rental_id=$1 ORDER BY id DESC`, id)
	if e != nil {
		return []any{}
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var i int64
		var typ, note string
		var amount float64
		var at time.Time
		if rows.Scan(&i, &typ, &amount, &note, &at) == nil {
			out = append(out, map[string]any{"id": i, "type": typ, "amount": amount, "note": note, "created_at": at})
		}
	}
	return out
}
func depositTransactions(ctx context.Context, db *pgxpool.Pool, id int64) []any {
	rows, e := db.Query(ctx, `SELECT id,transaction_type,amount,note,created_at FROM deposit_transactions WHERE rental_id=$1 ORDER BY id DESC`, id)
	if e != nil {
		return []any{}
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var i int64
		var typ, note string
		var amount float64
		var at time.Time
		if rows.Scan(&i, &typ, &amount, &note, &at) == nil {
			out = append(out, map[string]any{"id": i, "type": typ, "amount": amount, "note": note, "created_at": at})
		}
	}
	return out
}
func rentalExtensionCount(ctx context.Context, db *pgxpool.Pool, id int64) int {
	var n int
	_ = db.QueryRow(ctx, `SELECT extension_count FROM rentals WHERE id=$1`, id).Scan(&n)
	return n
}

func rentalEvents(ctx context.Context, db *pgxpool.Pool, id int64) []any {
	rows, e := db.Query(ctx, `SELECT id,event_type,actor_role,from_status,to_status,payload,created_at FROM rental_events WHERE rental_id=$1 ORDER BY id DESC`, id)
	if e != nil {
		return []any{}
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var eid int64
		var typ, role, from, to string
		var payload []byte
		var at time.Time
		if rows.Scan(&eid, &typ, &role, &from, &to, &payload, &at) == nil {
			var p any
			_ = json.Unmarshal(payload, &p)
			out = append(out, map[string]any{"id": eid, "type": typ, "actor_role": role, "from": from, "to": to, "payload": p, "created_at": at})
		}
	}
	return out
}
func rentalExtras(ctx context.Context, db *pgxpool.Pool, id int64) []any {
	rows, e := db.Query(ctx, `SELECT id,name,qty,unit_price,total FROM rental_extras WHERE rental_id=$1 ORDER BY id DESC`, id)
	if e != nil {
		return []any{}
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var x int64
		var n string
		var q int
		var u, t float64
		if rows.Scan(&x, &n, &q, &u, &t) == nil {
			out = append(out, map[string]any{"id": x, "name": n, "qty": q, "unit_price": u, "total": t})
		}
	}
	return out
}
func rentalPayments(ctx context.Context, db *pgxpool.Pool, id int64) []any {
	rows, e := db.Query(ctx, `SELECT id,payment_type,status,amount,provider,created_at FROM rental_payments WHERE rental_id=$1 ORDER BY id DESC`, id)
	if e != nil {
		return []any{}
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var x int64
		var typ, st, prov string
		var amt float64
		var at time.Time
		if rows.Scan(&x, &typ, &st, &amt, &prov, &at) == nil {
			out = append(out, map[string]any{"id": x, "type": typ, "status": st, "amount": amt, "provider": prov, "created_at": at})
		}
	}
	return out
}
func rentalInspections(ctx context.Context, db *pgxpool.Pool, id int64) []any {
	rows, e := db.Query(ctx, `SELECT id,kind,mileage,fuel_level,notes,photos,created_at FROM rental_inspections WHERE rental_id=$1 ORDER BY id DESC`, id)
	if e != nil {
		return []any{}
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var x int64
		var k, n string
		var m, f *int
		var ph []byte
		var at time.Time
		if rows.Scan(&x, &k, &m, &f, &n, &ph, &at) == nil {
			var p any
			_ = json.Unmarshal(ph, &p)
			out = append(out, map[string]any{"id": x, "kind": k, "mileage": m, "fuel_level": f, "notes": n, "photos": p, "created_at": at})
		}
	}
	return out
}
func rentalExpenses(ctx context.Context, db *pgxpool.Pool, id int64) []any {
	rows, e := db.Query(ctx, `SELECT id,expense_type,amount,note,created_at FROM rental_expenses WHERE rental_id=$1 ORDER BY id DESC`, id)
	if e != nil {
		return []any{}
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var x int64
		var k, n string
		var a float64
		var at time.Time
		if rows.Scan(&x, &k, &a, &n, &at) == nil {
			out = append(out, map[string]any{"id": x, "type": k, "amount": a, "note": n, "created_at": at})
		}
	}
	return out
}

func maxInt(v, d int) int {
	if v <= 0 {
		return d
	}
	return v
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func (a *App) verification(w http.ResponseWriter, r *http.Request) {
	owner := userID(r.Context())
	if r.Method == "POST" {
		var in struct{ Name, Phone string }
		if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" {
			write(w, 422, map[string]string{"error": "имя клиента обязательно"})
			return
		}
		var id int64
		err := a.db.QueryRow(r.Context(), `INSERT INTO verifications(owner_id,user_id,stage,score,status,progress,risk_level,client_phone)
			VALUES($1,$1,'Анкета',72,'review',20,'medium',$2) RETURNING id`, owner, normalizePhone(in.Phone)).Scan(&id)
		if err != nil {
			write(w, 500, map[string]string{"error": "не удалось создать проверку"})
			return
		}
		_, _ = a.db.Exec(r.Context(), `INSERT INTO rentals(owner_id,car_name,client_name,client_phone,status,amount) VALUES($1,'Новая заявка',$2,$3,'review',0)`, owner, in.Name, normalizePhone(in.Phone))
		write(w, 201, map[string]any{"id": id, "ok": true})
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT id,stage,score,status,progress,risk_level,client_phone,created_at FROM verifications WHERE owner_id=$1 ORDER BY id DESC`, owner)
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var stage, status, risk, phone string
		var score, progress int
		var created time.Time
		if rows.Scan(&id, &stage, &score, &status, &progress, &risk, &phone, &created) == nil {
			out = append(out, map[string]any{"id": id, "stage": stage, "score": score, "status": status, "progress": progress, "risk_level": risk, "phone": phone, "created_at": created})
		}
	}
	write(w, 200, out)
}

func (a *App) verificationByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/verification/"), 10, 64)
	if err != nil {
		write(w, 400, map[string]string{"error": "bad id"})
		return
	}
	if r.Method != "PATCH" {
		write(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var in struct {
		Stage, Status, RiskLevel string
		Score, Progress          int
	}
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	if in.Progress < 0 {
		in.Progress = 0
	}
	if in.Progress > 100 {
		in.Progress = 100
	}
	_, err = a.db.Exec(r.Context(), `UPDATE verifications SET stage=COALESCE(NULLIF($1,''),stage),status=COALESCE(NULLIF($2,''),status),risk_level=COALESCE(NULLIF($3,''),risk_level),score=$4,progress=$5 WHERE id=$6 AND owner_id=$7`,
		in.Stage, in.Status, in.RiskLevel, in.Score, in.Progress, id, userID(r.Context()))
	if err != nil {
		write(w, 500, map[string]string{"error": "не удалось обновить проверку"})
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *App) clients(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT client_name,MAX(client_phone),COUNT(*),COALESCE(SUM(amount),0),MAX(created_at) FROM rentals WHERE owner_id=$1 GROUP BY client_name ORDER BY MAX(created_at) DESC`, userID(r.Context()))
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var name, phone string
		var count int
		var total float64
		var last time.Time
		if rows.Scan(&name, &phone, &count, &total, &last) == nil {
			out = append(out, map[string]any{"name": name, "phone": phone, "rentals": count, "total": total, "last": last})
		}
	}
	write(w, 200, out)
}

func (a *App) notifications(w http.ResponseWriter, r *http.Request) {
	id := userID(r.Context())
	var pending, review, maintenance int
	_ = a.db.QueryRow(r.Context(), `SELECT count(*) FROM rentals WHERE owner_id=$1 AND status='pending'`, id).Scan(&pending)
	_ = a.db.QueryRow(r.Context(), `SELECT count(*) FROM verifications WHERE owner_id=$1 AND status IN ('review','pending')`, id).Scan(&review)
	_ = a.db.QueryRow(r.Context(), `SELECT count(*) FROM cars WHERE owner_id=$1 AND status='maintenance'`, id).Scan(&maintenance)
	items := []map[string]any{}
	if pending > 0 {
		items = append(items, map[string]any{"type": "rental", "title": fmt.Sprintf("%d новых заявок на аренду", pending), "text": "Проверьте клиента и подтвердите условия."})
	}
	if review > 0 {
		items = append(items, map[string]any{"type": "verification", "title": fmt.Sprintf("%d проверки требуют внимания", review), "text": "Завершите пред-проверку клиента перед договором."})
	}
	if maintenance > 0 {
		items = append(items, map[string]any{"type": "car", "title": fmt.Sprintf("%d авто на обслуживании", maintenance), "text": "Проверьте готовность машин к выдаче."})
	}
	write(w, 200, items)
}

func (a *App) leads(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, Phone, Email string }
	if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Phone) == "" {
		write(w, 422, map[string]string{"error": "name and phone are required"})
		return
	}
	_, err := a.db.Exec(r.Context(), `INSERT INTO leads(name,phone,email) VALUES($1,$2,NULLIF($3,''))`, strings.TrimSpace(in.Name), normalizePhone(in.Phone), strings.TrimSpace(in.Email))
	if err != nil {
		write(w, 500, map[string]string{"error": "could not save lead"})
		return
	}
	write(w, 201, map[string]bool{"ok": true})
}

func (a *App) customerRegister(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, Phone, Email, Password string }
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Phone = normalizePhone(strings.TrimSpace(in.Phone))
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Name == "" || len(in.Phone) < 12 || len(in.Password) < 8 {
		write(w, 422, map[string]string{"error": "имя, телефон и пароль (8+ символов) обязательны"})
		return
	}
	if in.Email != "" && !validEmail(in.Email) {
		write(w, 422, map[string]string{"error": "некорректный email"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	var id int64
	err := a.db.QueryRow(r.Context(), `INSERT INTO users(name,phone,email,password_hash,role,city,company_name) VALUES($1,$2,NULLIF($3,''),$4,'customer','', '') RETURNING id`, in.Name, in.Phone, in.Email, string(hash)).Scan(&id)
	if err != nil {
		write(w, 409, map[string]string{"error": "телефон или email уже зарегистрирован"})
		return
	}
	t, _ := a.token(id, "customer")
	write(w, 201, map[string]any{"token": t, "user": map[string]any{"id": id, "name": in.Name, "phone": in.Phone, "email": in.Email, "role": "customer"}})
}

func (a *App) customerLogin(w http.ResponseWriter, r *http.Request) {
	var in struct{ Identifier, Password string }
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	idf := strings.TrimSpace(in.Identifier)
	phone := normalizePhone(idf)
	var u User
	var hash string
	err := a.db.QueryRow(r.Context(), `SELECT id,name,COALESCE(email,''),COALESCE(phone,''),phone_verified,role,city,company_name,password_hash FROM users WHERE role='customer' AND (email=lower($1) OR phone=$2)`, idf, phone).
		Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.PhoneVerified, &u.Role, &u.City, &u.CompanyName, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		write(w, 401, map[string]string{"error": "неверный телефон/email или пароль"})
		return
	}
	t, _ := a.token(u.ID, u.Role)
	write(w, 200, map[string]any{"token": t, "user": u})
}

func parseRangeValues(r *http.Request) (time.Time, time.Time, error) {
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from == "" || to == "" {
		return time.Time{}, time.Time{}, errors.New("dates required")
	}
	parse := func(v string) (time.Time, error) {
		if t, e := time.Parse(time.RFC3339, v); e == nil {
			return t, nil
		}
		return time.Parse("2006-01-02", v)
	}
	st, e := parse(from)
	if e != nil {
		return time.Time{}, time.Time{}, e
	}
	en, e := parse(to)
	if e != nil {
		return time.Time{}, time.Time{}, e
	}
	if !en.After(st) {
		return time.Time{}, time.Time{}, errors.New("invalid date range")
	}
	return st, en, nil
}

func (a *App) publicFleets(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	city := strings.TrimSpace(r.URL.Query().Get("city"))
	rows, err := a.db.Query(r.Context(), `SELECT fp.id,fp.slug,fp.title,fp.description,fp.city,fp.rating,u.name,COUNT(c.id) FILTER (WHERE c.public_enabled AND c.status='available') FROM fleet_profiles fp JOIN users u ON u.id=fp.owner_id LEFT JOIN cars c ON c.owner_id=fp.owner_id WHERE fp.published AND ($1='' OR fp.city ILIKE '%'||$1||'%' OR fp.title ILIKE '%'||$1||'%') AND ($2='' OR fp.title ILIKE '%'||$2||'%' OR u.name ILIKE '%'||$2||'%') GROUP BY fp.id,u.name ORDER BY fp.rating DESC,fp.id`, city, q)
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var slug, title, desc, fcity, owner string
		var rating float64
		var count int
		if rows.Scan(&id, &slug, &title, &desc, &fcity, &rating, &owner, &count) == nil {
			out = append(out, map[string]any{"id": id, "slug": slug, "title": title, "description": desc, "city": fcity, "rating": rating, "owner": owner, "available_cars": count})
		}
	}
	write(w, 200, out)
}

func (a *App) publicCars(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	city := strings.TrimSpace(r.URL.Query().Get("city"))
	var st, en time.Time
	if r.URL.Query().Get("from") != "" || r.URL.Query().Get("to") != "" {
		var err error
		st, en, err = parseRangeValues(r)
		if err != nil {
			write(w, 422, map[string]string{"error": "некорректный диапазон дат"})
			return
		}
	}
	rows, err := a.db.Query(r.Context(), `SELECT c.id,c.owner_id,c.brand,c.model,c.year,c.location,c.daily_price,c.mileage,c.color,c.category,c.seats,c.transmission,c.fuel,c.description,c.image_url,c.deposit,fp.slug,fp.title,fp.city,fp.rating,u.name FROM cars c JOIN fleet_profiles fp ON fp.owner_id=c.owner_id JOIN users u ON u.id=c.owner_id WHERE c.public_enabled AND fp.published AND c.status='available' AND ($1='' OR c.location ILIKE '%'||$1||'%' OR fp.city ILIKE '%'||$1||'%') AND ($2='' OR (c.brand||' '||c.model||' '||c.category||' '||fp.title) ILIKE '%'||$2||'%') AND ($3::timestamptz IS NULL OR NOT EXISTS (SELECT 1 FROM rentals r WHERE r.car_id=c.id AND r.status IN ('hold','pending','review','confirmed','active') AND (r.status<>'hold' OR r.hold_expires_at IS NULL OR r.hold_expires_at>now()) AND r.starts_at < $4 AND r.ends_at > $3)) ORDER BY c.daily_price`, city, q, nullableTime(st), en)
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, ownerID int64
		var brand, model, loc, color, cat, trans, fuel, desc, img, slug, title, fcity, owner string
		var year, seats, mileage int
		var price, deposit, rating float64
		if rows.Scan(&id, &ownerID, &brand, &model, &year, &loc, &price, &mileage, &color, &cat, &seats, &trans, &fuel, &desc, &img, &deposit, &slug, &title, &fcity, &rating, &owner) == nil {
			out = append(out, map[string]any{"id": id, "owner_id": ownerID, "brand": brand, "model": model, "year": year, "location": loc, "daily_price": price, "mileage": mileage, "color": color, "category": cat, "seats": seats, "transmission": trans, "fuel": fuel, "description": desc, "image_url": img, "deposit": deposit, "fleet_slug": slug, "fleet_title": title, "fleet_city": fcity, "fleet_rating": rating, "owner": owner})
		}
	}
	write(w, 200, out)
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func (a *App) publicCar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/public/cars/"), 10, 64)
	if err != nil {
		write(w, 400, map[string]string{"error": "bad id"})
		return
	}
	var c map[string]any
	var brand, model, loc, color, cat, trans, fuel, desc, img, slug, title, fcity, owner string
	var year, seats, mileage int
	var price, deposit, rating float64
	err = a.db.QueryRow(r.Context(), `SELECT c.brand,c.model,c.year,c.location,c.daily_price,c.mileage,c.color,c.category,c.seats,c.transmission,c.fuel,c.description,c.image_url,c.deposit,fp.slug,fp.title,fp.city,fp.rating,u.name FROM cars c JOIN fleet_profiles fp ON fp.owner_id=c.owner_id JOIN users u ON u.id=c.owner_id WHERE c.id=$1 AND c.public_enabled AND fp.published`, id).Scan(&brand, &model, &year, &loc, &price, &mileage, &color, &cat, &seats, &trans, &fuel, &desc, &img, &deposit, &slug, &title, &fcity, &rating, &owner)
	if err != nil {
		write(w, 404, map[string]string{"error": "автомобиль не найден"})
		return
	}
	c = map[string]any{"id": id, "brand": brand, "model": model, "year": year, "location": loc, "daily_price": price, "mileage": mileage, "color": color, "category": cat, "seats": seats, "transmission": trans, "fuel": fuel, "description": desc, "image_url": img, "deposit": deposit, "fleet_slug": slug, "fleet_title": title, "fleet_city": fcity, "fleet_rating": rating, "owner": owner}
	write(w, 200, c)
}

func (a *App) publicAvailabilityDates(w http.ResponseWriter, r *http.Request) {
	carID, err := strconv.ParseInt(r.URL.Query().Get("car_id"), 10, 64)
	if err != nil {
		write(w, 422, map[string]string{"error": "car_id required"})
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		write(w, 422, map[string]string{"error": "from and to required"})
		return
	}
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		write(w, 422, map[string]string{"error": "invalid from"})
		return
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil || !end.After(start) {
		write(w, 422, map[string]string{"error": "invalid to"})
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT to_char(gs,'YYYY-MM-DD') FROM rentals r CROSS JOIN LATERAL generate_series(date(r.starts_at), date(r.ends_at) - 1, interval '1 day') gs WHERE r.car_id=$1 AND r.status IN ('hold','pending','review','confirmed','preparing','active') AND (r.status<>'hold' OR r.hold_expires_at IS NULL OR r.hold_expires_at>now()) AND gs >= $2::date AND gs < $3::date ORDER BY gs`, carID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	blocked := []string{}
	for rows.Next() {
		var d string
		if rows.Scan(&d) == nil {
			blocked = append(blocked, d)
		}
	}
	write(w, 200, map[string]any{"blocked": blocked, "from": from, "to": to})
}

func (a *App) publicAvailability(w http.ResponseWriter, r *http.Request) {
	carID, err := strconv.ParseInt(r.URL.Query().Get("car_id"), 10, 64)
	if err != nil {
		write(w, 422, map[string]string{"error": "car_id required"})
		return
	}
	st, en, err := parseRangeValues(r)
	if err != nil {
		write(w, 422, map[string]string{"error": "некорректные даты"})
		return
	}
	var blocked bool
	err = a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM rentals WHERE car_id=$1 AND status IN ('hold','pending','review','confirmed','active') AND (status<>'hold' OR hold_expires_at IS NULL OR hold_expires_at>now()) AND starts_at < $3 AND ends_at > $2)`, carID, st, en).Scan(&blocked)
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	write(w, 200, map[string]any{"available": !blocked, "from": st, "to": en})
}

func (a *App) fleetProfile(w http.ResponseWriter, r *http.Request) {
	owner := userID(r.Context())
	if r.Method == "PATCH" || r.Method == "POST" {
		var in struct {
			Slug        string `json:"slug"`
			Title       string `json:"title"`
			Description string `json:"description"`
			City        string `json:"city"`
			Published   bool   `json:"published"`
		}
		if decode(r, &in) != nil {
			write(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		in.Slug = strings.TrimSpace(in.Slug)
		if in.Slug == "" {
			in.Slug = "fleet-" + strconv.FormatInt(owner, 10)
		}
		if in.Title == "" {
			in.Title = "Мой автопарк"
		}
		city := first(in.City, "Санкт-Петербург")
		_, err := a.db.Exec(r.Context(), `INSERT INTO fleet_profiles(owner_id,slug,title,description,city,published,updated_at) VALUES($1,$2,$3,$4,$5,$6,now()) ON CONFLICT(owner_id) DO UPDATE SET slug=EXCLUDED.slug,title=EXCLUDED.title,description=EXCLUDED.description,city=EXCLUDED.city,published=EXCLUDED.published,updated_at=now()`, owner, in.Slug, in.Title, strings.TrimSpace(in.Description), city, in.Published)
		if err != nil {
			write(w, 409, map[string]string{"error": "не удалось сохранить страницу автопарка; slug должен быть уникальным"})
			return
		}
		// The storefront is another view of the same owner profile.
		_, err = a.db.Exec(r.Context(), `UPDATE users SET city=$1,company_name=$2,updated_at=now() WHERE id=$3`, city, in.Title, owner)
		if err != nil {
			write(w, 500, map[string]string{"error": "не удалось синхронизировать профиль владельца"})
			return
		}
	}
	var id int64
	var slug, title, desc, city string
	var published bool
	var rating float64
	err := a.db.QueryRow(r.Context(), `SELECT id,slug,title,description,city,published,rating FROM fleet_profiles WHERE owner_id=$1`, owner).Scan(&id, &slug, &title, &desc, &city, &published, &rating)
	if err != nil {
		_, _ = a.db.Exec(r.Context(), `INSERT INTO fleet_profiles(owner_id,slug,title,city,published) SELECT $1,$2,$3,COALESCE(NULLIF(city,''),'Санкт-Петербург'),true FROM users WHERE id=$1`, owner, "fleet-"+strconv.FormatInt(owner, 10), "Мой автопарк")
		a.db.QueryRow(r.Context(), `SELECT id,slug,title,description,city,published,rating FROM fleet_profiles WHERE owner_id=$1`, owner).Scan(&id, &slug, &title, &desc, &city, &published, &rating)
	}
	write(w, 200, map[string]any{"id": id, "slug": slug, "title": title, "description": desc, "city": city, "published": published, "rating": rating})
}

func (a *App) bookings(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		write(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if r.Context().Value(ctxKey("role")) != "customer" {
		write(w, 403, map[string]string{"error": "customer access required"})
		return
	}
	var in struct {
		CarID           int64  `json:"car_id"`
		StartsAt        string `json:"starts_at"`
		EndsAt          string `json:"ends_at"`
		PickupLocation  string `json:"pickup_location"`
		DropoffLocation string `json:"dropoff_location"`
	}
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	st, en, err := parseBookingTimes(in.StartsAt, in.EndsAt)
	if err != nil {
		write(w, 422, map[string]string{"error": "укажите корректные даты аренды"})
		return
	}
	if !st.After(time.Now().Add(-5 * time.Minute)) {
		write(w, 422, map[string]string{"error": "дата начала уже прошла"})
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		write(w, 500, map[string]string{"error": "transaction error"})
		return
	}
	defer tx.Rollback(r.Context())
	var ownerID int64
	var carName string
	var price, deposit float64
	var status string
	err = tx.QueryRow(r.Context(), `SELECT owner_id,brand||' '||model,daily_price,deposit,status FROM cars WHERE id=$1 AND public_enabled FOR UPDATE`, in.CarID).Scan(&ownerID, &carName, &price, &deposit, &status)
	if err != nil {
		write(w, 404, map[string]string{"error": "автомобиль не найден"})
		return
	}
	if status != "available" {
		write(w, 409, map[string]string{"error": "автомобиль сейчас недоступен"})
		return
	}
	var overlap bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM rentals WHERE car_id=$1 AND status IN ('hold','pending','review','confirmed','active') AND (status<>'hold' OR hold_expires_at IS NULL OR hold_expires_at>now()) AND starts_at < $3 AND ends_at > $2)`, in.CarID, st, en).Scan(&overlap)
	if err != nil {
		write(w, 500, map[string]string{"error": "не удалось проверить доступность"})
		return
	}
	if overlap {
		write(w, 409, map[string]string{"error": "эти даты уже заняты"})
		return
	}
	days := int(math.Ceil(en.Sub(st).Hours() / 24))
	if days < 1 {
		days = 1
	}
	subtotal := price * float64(days)
	customer := userID(r.Context())
	var id int64
	err = tx.QueryRow(r.Context(), `INSERT INTO rentals(owner_id,car_id,client_user_id,car_name,client_name,client_phone,status,amount,subtotal,deposit,starts_at,ends_at,source,payment_status,pickup_location,dropoff_location) SELECT $1,$2,$3,$4,u.name,u.phone,'pending',$5,$5,$6,$7,$8,'marketplace','unpaid',$9,$10 FROM users u WHERE u.id=$3 RETURNING id`, ownerID, in.CarID, customer, carName, subtotal, deposit, st, en, first(in.PickupLocation, ""), first(in.DropoffLocation, "")).Scan(&id)
	if err != nil {
		write(w, 500, map[string]string{"error": "не удалось создать бронирование"})
		return
	}
	code := fmt.Sprintf("KEY-%06d", id)
	_, _ = tx.Exec(r.Context(), `UPDATE rentals SET booking_code=$1,final_total=$2,updated_at=now() WHERE id=$3`, code, subtotal, id)
	_, _ = tx.Exec(r.Context(), `INSERT INTO rental_events(rental_id,actor_id,actor_role,event_type,from_status,to_status,payload) VALUES($1,$2,'customer','booking_created',NULL,'pending',$3::jsonb)`, id, customer, mustJSON(map[string]any{"source": "marketplace"}))
	if deposit > 0 {
		_, _ = tx.Exec(r.Context(), `INSERT INTO deposit_transactions(rental_id,transaction_type,amount,note) VALUES($1,'hold',$2,'Депозит по бронированию')`, id, deposit)
	}
	if err = tx.Commit(r.Context()); err != nil {
		write(w, 500, map[string]string{"error": "не удалось подтвердить бронь"})
		return
	}
	write(w, 201, map[string]any{"id": id, "booking_code": code, "status": "pending", "car": carName, "starts_at": st, "ends_at": en, "subtotal": subtotal, "deposit": deposit, "payment_status": "unpaid"})
}

func parseBookingTimes(from, to string) (time.Time, time.Time, error) {
	parse := func(v string) (time.Time, error) {
		if t, e := time.Parse(time.RFC3339, v); e == nil {
			return t, nil
		}
		return time.Parse("2006-01-02", v)
	}
	st, e := parse(from)
	if e != nil {
		return time.Time{}, time.Time{}, e
	}
	en, e := parse(to)
	if e != nil {
		return time.Time{}, time.Time{}, e
	}
	if !en.After(st) {
		return time.Time{}, time.Time{}, errors.New("range")
	}
	return st, en, nil
}

func (a *App) customerBookings(w http.ResponseWriter, r *http.Request) {
	if r.Context().Value(ctxKey("role")) != "customer" {
		write(w, 403, map[string]string{"error": "customer access required"})
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT r.id,COALESCE(r.booking_code,''),r.car_name,r.status,r.amount,r.deposit,r.starts_at,r.ends_at,COALESCE(fp.title,''),COALESCE(fp.city,''),r.payment_status,r.pickup_meeting_at,r.pickup_meeting_location,r.return_meeting_at,r.return_meeting_location FROM rentals r LEFT JOIN cars c ON c.id=r.car_id LEFT JOIN fleet_profiles fp ON fp.owner_id=r.owner_id WHERE r.client_user_id=$1 ORDER BY r.starts_at DESC NULLS LAST,r.id DESC`, userID(r.Context()))
	if err != nil {
		write(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var code, car, status, fleet, city, payment string
		var amount, deposit float64
		var st, en, pickupMeeting, returnMeeting *time.Time
		var pickupLocation, returnLocation string
		if rows.Scan(&id, &code, &car, &status, &amount, &deposit, &st, &en, &fleet, &city, &payment, &pickupMeeting, &pickupLocation, &returnMeeting, &returnLocation) == nil {
			out = append(out, map[string]any{"id": id, "booking_code": code, "car": car, "status": status, "amount": amount, "deposit": deposit, "starts_at": st, "ends_at": en, "fleet": fleet, "city": city, "payment_status": payment, "pickup_meeting_at": pickupMeeting, "pickup_meeting_location": pickupLocation, "return_meeting_at": returnMeeting, "return_meeting_location": returnLocation})
		}
	}
	write(w, 200, out)
}

func (a *App) bookingByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/bookings/"), 10, 64)
	if err != nil {
		write(w, 400, map[string]string{"error": "bad id"})
		return
	}
	if r.Method != "PATCH" {
		write(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var in struct{ Status, Reason string }
	if decode(r, &in) != nil {
		write(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	role, _ := r.Context().Value(ctxKey("role")).(string)
	if role == "customer" {
		if in.Status != "cancelled" {
			write(w, 422, map[string]string{"error": "customer can only cancel"})
			return
		}
		var ownerID int64
		if err := a.db.QueryRow(r.Context(), `SELECT owner_id FROM rentals WHERE id=$1 AND client_user_id=$2 AND status IN ('hold','pending','confirmed')`, id, userID(r.Context())).Scan(&ownerID); err != nil {
			write(w, 404, map[string]string{"error": "бронирование не найдено или уже началось"})
			return
		}
		if err := a.transitionRental(r.Context(), id, userID(r.Context()), "customer", "cancelled", in.Reason); err != nil {
			write(w, 409, map[string]string{"error": err.Error()})
			return
		}
		write(w, 200, map[string]bool{"ok": true})
		return
	}
	if role != "owner" {
		write(w, 403, map[string]string{"error": "access denied"})
		return
	}
	if !contains([]string{"confirmed", "active", "completed", "cancelled", "rejected"}, in.Status) {
		write(w, 422, map[string]string{"error": "invalid status"})
		return
	}
	_, err = a.db.Exec(r.Context(), `UPDATE rentals SET status=$1,cancellation_reason=$2 WHERE id=$3 AND owner_id=$4`, in.Status, in.Reason, id, userID(r.Context()))
	if err != nil {
		write(w, 500, map[string]string{"error": "не удалось обновить бронь"})
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := strings.Split(getenv("CORS_ORIGIN", "http://localhost:5173"), ",")
		for _, a := range allowed {
			if strings.TrimSpace(a) == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				break
			}
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(st).Round(time.Millisecond))
	})
}
