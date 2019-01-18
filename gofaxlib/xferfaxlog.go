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
	Ts       time.Time
	Commid   string
	Modem    string
	Jobid    uint
	Jobtag   string
	Filename string
	Sender   string
	Destnum  string
	RemoteID string
	Params   uint
	Pages    uint
	Jobtime  time.Duration
	Conntime time.Duration
	Reason   string
	Cidname  string
	Cidnum   string
	Owner    string
	Dcs      string
}

// NewXFRecord creates a new xferfaxlog record for a FaxResult
func NewXFRecord(result *FaxResult) *XFRecord {
	duration := result.EndTs.Sub(result.StartTs)

	r := &XFRecord{
		Ts:       result.StartTs,
		Commid:   result.sessionlog.CommID(),
		RemoteID: result.RemoteID,
		Params:   EncodeParams(result.TransferRate, result.Ecm),
		Pages:    result.TransferredPages,
		Jobtime:  duration,
		Conntime: duration,
		Reason:   result.ResultText,
	}

	if len(result.PageResults) > 0 {
		r.Dcs = result.PageResults[0].EncodingName
	}

	return r
}

func (r *XFRecord) formatTransmissionReport() string {
	return fmt.Sprintf(xLogFormat, r.Ts.Format(tsLayout), "SEND", r.Commid, r.Modem,
		r.Jobid, r.Jobtag, r.Sender, r.Destnum, r.RemoteID, r.Params, r.Pages,
		formatDuration(r.Jobtime), formatDuration(r.Conntime), r.Reason, "", "", "", r.Owner, r.Dcs)
}

func (r *XFRecord) formatReceptionReport() string {
	return fmt.Sprintf(xLogFormat, r.Ts.Format(tsLayout), "RECV", r.Commid, r.Modem,
		r.Filename, "", "fax", r.Destnum, r.RemoteID, r.Params, r.Pages,
		formatDuration(r.Jobtime), formatDuration(r.Conntime), r.Reason,
		fmt.Sprintf("\"%s\"", r.Cidname), fmt.Sprintf("\"%s\"", r.Cidnum), "", "", r.Dcs)
}

// SaveTransmissionReport appends a transmisison record to the configured xferfaxlog file
func (r *XFRecord) SaveTransmissionReport() error {
	if Config.Hylafax.Xferfaxlog == "" {
		return nil
	}
	return AppendTo(Config.Hylafax.Xferfaxlog, r.formatTransmissionReport())
}

// SaveTxCdrToDB adds a transmisison record to the mysql database
func (r *XFRecord) SaveTxCdrToDB() error {
	if Config.MySQL.Host == "" && Config.MySQL.User == "" && Config.MySQL.Pass == "" && Config.MySQL.Database == "" {
		return nil
	}

	return log_cdr_db()
}

// SaveReceptionReport appends a reception record to the configured xferfaxlog file
func (r *XFRecord) SaveReceptionReport() error {
	if Config.Hylafax.Xferfaxlog == "" {
		return nil
	}
	return AppendTo(Config.Hylafax.Xferfaxlog, r.formatReceptionReport())
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

func log_cdr_db() error {
	db, err := sql.Open("mysql", Config.MySQL.User+":"+Config.MySQL.Pass+"@tcp("+Config.MySQL.Host+":"+Config.MySQL.Port+")/"+Config.MySQL.Database+"?charset="+Config.MySQL.Charset)
	if err != nil {
		log.Fatal("Cannot open DB connection", err)
	}
	// defer the close till after the main function has finished
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO xferfaxlog (timestamp, entrytype, commid, modem, jobid, jobtag, user, localnumber, tsi, params, npages, jobtime, conntime, reason, cidname, cidnumber, callid, owner, dcs) VALUES ( ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? )")
	if err != nil {
		log.Fatal("Cannot prepare DB statement", err)
	}
	// Close the statement when we leave main()
	defer stmt.Close()

	_, err = stmt.Exec("2019-01-15 17:39:00", "SEND", "00002339", "freeswitch4", "122", "", "a.bedhiafi@netcom-group.fr", "33184039108", "", "65576", 3, "00:01:06", "00:01:06", "OK", "", "", "", "faxmaster", "T.85")
	if err != nil {
		log.Fatal("Cannot run insert statement", err)
		return nil
	} else {
		return err
	}
}
