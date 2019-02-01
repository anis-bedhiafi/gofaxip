// This file is part of the GOfax.IP project - https://github.com/gonicus/gofaxip
// Copyright (C) 2014 GONICUS GmbH, Germany - http://www.gonicus.de
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; version 2
// of the License.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software
// Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.

package gofaxlib

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	// 19 fields
	xLogFormat = "%s\t%s\t%s\t%s\t%v\t\"%s\"\t%s\t\"%s\"\t\"%s\"\t%d\t%d\t%s\t%s\t\"%s\"\t\"%s\"\t\"%s\"\t\"%s\"\t\"%s\"\t\"%s\""
	tsLayout   = "01/02/06 15:04"
)

// XFRecord holds all data for a HylaFAX xferfaxlog record
type XFRecord struct {
	Ts           time.Time
	Commid       string
	Modem        string
	Jobid        uint
	Jobtag       string
	Filename     string
	Sender       string
	Destnum      string
	RemoteID     string
	TransferRate uint
	Ecm          bool
	Params       uint
	Pages        uint
	Jobtime      time.Duration
	Conntime     time.Duration
	Reason       string
	Cidname      string
	Cidnum       string
	Callid       string
	Owner        string
	Dcs          string
}

// NewXFRecord creates a new xferfaxlog record for a FaxResult
func NewXFRecord(result *FaxResult) *XFRecord {
	duration := result.EndTs.Sub(result.StartTs)

	r := &XFRecord{
		Ts:           result.StartTs,
		Commid:       result.sessionlog.CommID(),
		TransferRate: result.TransferRate,
		Ecm:          result.Ecm,
		Params:       EncodeParams(result.TransferRate, result.Ecm),
		Pages:        result.TransferredPages,
		Jobtime:      duration,
		Conntime:     duration,
		Reason:       result.ResultText,
	}

	if len(result.PageResults) > 0 {
		r.Dcs = result.PageResults[0].EncodingName
	}

	return r
}

func (r *XFRecord) formatTransmissionReport() string {
	return fmt.Sprintf(xLogFormat, r.Ts.Format(tsLayout), "SEND", r.Commid, r.Modem,
		r.Jobid, r.Jobtag, r.Sender, r.Destnum, r.RemoteID, r.TransferRate, r.Ecm, r.Params, r.Pages,
		formatDuration(r.Jobtime), formatDuration(r.Conntime), r.Reason, r.Cidname, r.Cidnum, r.Callid, r.Owner, r.Dcs)
}

func (r *XFRecord) formatReceptionReport() string {
	return fmt.Sprintf(xLogFormat, r.Ts.Format(tsLayout), "RECV", r.Commid, r.Modem,
		r.Filename, "", "fax", r.Destnum, r.RemoteID, r.TransferRate, r.Ecm, r.Params, r.Pages,
		formatDuration(r.Jobtime), formatDuration(r.Conntime), r.Reason,
		fmt.Sprintf("\"%s\"", r.Cidname), fmt.Sprintf("\"%s\"", r.Cidnum), r.Callid, "", r.Dcs)
}

// SaveTransmissionReport appends a transmisison record to the configured xferfaxlog file
func (r *XFRecord) SaveTransmissionReport() error {
	if Config.Log.Xferfaxlog == "" {
		return nil
	}
	return AppendTo(Config.Log.Xferfaxlog, r.formatTransmissionReport())
}

// SaveReceptionReport appends a reception record to the configured xferfaxlog file
func (r *XFRecord) SaveReceptionReport() error {
	if Config.Log.Xferfaxlog == "" {
		return nil
	}
	return AppendTo(Config.Log.Xferfaxlog, r.formatReceptionReport())
}

// SaveTxCdrToDB adds a transmisison record to the mysql database
func (r *XFRecord) SaveTxCdrToDB() error {
	db, err := DBConnect()
	defer db.Close()

	if db != nil {
		stmt, err := db.Prepare("INSERT INTO xferfaxlog (timestamp, entrytype, commid, modem, jobid, jobtag, user, destnumber, tsi, transferrate, ecm, params, npages, jobtime, conntime, reason, cidname, cidnumber, callid, owner, dcs) VALUES ( ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? )")
		if err != nil {
			log.Fatal("Cannot prepare DB statement", err)
		}
		defer stmt.Close()

		_, err = stmt.Exec(r.Ts, "SEND", r.Commid, r.Modem, r.Jobid, r.Jobtag, r.Sender, r.Destnum, r.Cidnum, r.TransferRate, r.Ecm, r.Params, r.Pages, formatDuration(r.Jobtime), formatDuration(r.Conntime), r.Reason, r.Cidname, r.Cidnum, r.Callid, r.Owner, r.Dcs)
		if err != nil {
			log.Fatal("Cannot execute query", err)
		}
	}
	return err
}

// SaveRxCdrToDB adds a reception record to the mysql database
func (r *XFRecord) SaveRxCdrToDB() error {
	db, err := DBConnect()
	defer db.Close()

	if db != nil {
		//get user's email
		useremail := r.Destnum
		if r.Destnum != "" {
			err = db.QueryRow("SELECT e.email from email_number en JOIN numbers n ON en.number_id = n.id JOIN emails e ON en.email_id = e.id WHERE n.faxnum = ? AND n.enabled = 1 AND en.shared = 0", r.Destnum).Scan(&useremail)
			if err != nil {
				log.Fatal(err)
			}
		}
		//get client_code
		// client_code := ""
		// err = db.QueryRow("SELECT u.client_code from users u JOIN numbers n ON n.user_id = u.id WHERE n.faxnum = ? AND u.enabled = 1 AND n.enabled = 1", r.Destnum).Scan(&client_code)
		// if err != nil {
		// 	log.Fatal(err)
		// }

		stmt, err := db.Prepare("INSERT INTO xferfaxlog (timestamp, entrytype, commid, modem, jobid, jobtag, user, destnumber, tsi, transferrate, ecm, params, npages, jobtime, conntime, reason, cidname, cidnumber, callid, owner, dcs) VALUES ( ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? )")
		if err != nil {
			log.Fatal("Cannot prepare DB statement", err)
		}
		defer stmt.Close()

		_, err = stmt.Exec(r.Ts, "RECV", r.Commid, r.Modem, r.Filename, "", useremail, r.Destnum, r.RemoteID, r.TransferRate, r.Ecm, r.Params, r.Pages, formatDuration(r.Jobtime), formatDuration(r.Conntime), r.Reason, r.Cidname, r.Cidnum, r.Callid, useremail, r.Dcs)
		if err != nil {
			log.Fatal("Cannot execute query", err)
		}
	}
	return err
}

func formatDuration(d time.Duration) string {
	s := uint(d.Seconds())

	hours := s / (60 * 60)
	minutes := (s / 60) - (60 * hours)
	seconds := s % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// EncodeParams encodes given baud rate and ecm status to
// the status byte used in HylaFAX's xferfaxlog.
// This only encodes bitrate and ECM use right now.
func EncodeParams(baudrate uint, ecm bool) uint {

	var br uint
	switch {
	case baudrate > 12000:
		br = 5
	case baudrate > 9600:
		br = 4
	case baudrate > 7200:
		br = 3
	case baudrate > 4800:
		br = 2
	case baudrate > 2400:
		br = 1
	}

	var ec uint
	if ecm {
		ec = 1
	}

	return (br << 3) | (ec << 16)
}

func DBConnect() (*sql.DB, error) {
	if Config.MySQL.Host == "" && Config.MySQL.User == "" && Config.MySQL.Pass == "" && Config.MySQL.Database == "" {
		log.Fatal("DB connection parameters missing")
		return nil, nil
	}

	charset := Config.MySQL.Charset
	if charset == "" {
		charset = "utf8"
	}

	db, err := sql.Open("mysql", Config.MySQL.User+":"+Config.MySQL.Pass+"@tcp("+Config.MySQL.Host+":"+Config.MySQL.Port+")/"+Config.MySQL.Database+"?charset="+charset)
	if err != nil {
		log.Fatal("Cannot open DB connection", err)
	}

	return db, err
}
