package models

import "time"

type User struct {
	ID        string    `db:"id" json:"id"`
	FirstName string    `db:"first_name" json:"first_name"`
	LastName  string    `db:"last_name" json:"last_name"`
	Phone     string    `db:"phone" json:"phone"`
	Email     string    `db:"email" json:"email"`
	Password  string    `db:"password" json:"-"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Appointment struct {
	ID               string    `db:"id" json:"id"`
	UserID           string    `db:"user_id" json:"user_id"`
	Doctor           string    `db:"doctor" json:"doctor"`
	DoctorSpeciality string    `db:"doctor_speciality" json:"doctor_speciality"`
	DoctorOffice     string    `db:"doctor_office" json:"doctor_office"`
	Date             time.Time `db:"date" json:"date"`
	Status           string    `db:"status" json:"status"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
}

type Doctor struct {
	ID         string `db:"id" json:"id"`
	Name       string `db:"name" json:"name"`
	Speciality string `db:"speciality" json:"speciality"`
	Office     string `db:"office" json:"office"`
	LunchStart int    `db:"lunch_start" json:"lunch_start"`
	LunchEnd   int    `db:"lunch_end" json:"lunch_end"`
}

type SlotInfo struct {
	Doctor    string `json:"doctor"`
	DateTime  string `json:"datetime"`
	Available bool   `json:"available"`
}