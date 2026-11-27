// Copyright (C) 2010 Mark Olsen
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; either version 2
// of the License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
//
// See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software
// Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.

package engine

import (
	"fmt"
	"sort"
	"strings"
)

// tableprint.c: prints a list of names in as many console columns as fit

type TablePrint struct {
	error_   int
	dosort   int
	array    []string
	numitems uint
}

func TablePrint_Begin(dosort int) *TablePrint {
	return &TablePrint{dosort: dosort}
}

func TablePrint_Do(tp *TablePrint) {
	var numcolumns, itemspercolumn, extraitems, totalwidth uint

	if tp.numitems == 0 {
		return
	}

	textcolumns := uint(Con_GetColumns())

	columns := make([]uint, tp.numitems)
	stringlengths := make([]uint, tp.numitems)

	for i := uint(0); i < tp.numitems; i++ {
		stringlengths[i] = uint(len(tp.array[i]))
	}

	for numcolumns = tp.numitems; numcolumns > 0; numcolumns-- {
		itemspercolumn = tp.numitems / numcolumns
		extraitems = tp.numitems - (itemspercolumn * numcolumns)

		totalwidth = 0
		item := uint(0)

		for i := uint(0); i < numcolumns; i++ {
			columns[i] = 0

			n := itemspercolumn
			if i < extraitems {
				n++
			}
			for j := uint(0); j < n; j++ {
				if stringlengths[item] > columns[i] {
					columns[i] = stringlengths[item]
				}

				item++
			}

			totalwidth += columns[i] + 2
		}

		if totalwidth <= textcolumns || numcolumns == 1 {
			break
		}
	}

	item := uint(0)

	for i := uint(0); i < itemspercolumn+1; i++ {
		var tempstr strings.Builder

		for j := uint(0); j < numcolumns && !(i == itemspercolumn && j >= extraitems); j++ {
			fmt.Fprintf(&tempstr, "%-*s  ", int(columns[j]), tp.array[item+j*itemspercolumn+min(j, extraitems)])
		}

		if tempstr.Len() != 0 {
			s := tempstr.String()
			Con_Print(s[:len(s)-2] + "\n")
		}

		item++
	}
}

func TablePrint_End(tp *TablePrint) {
	if tp != nil && tp.error_ == 0 {
		if tp.dosort != 0 {
			sort.Strings(tp.array[:tp.numitems])
		}

		TablePrint_Do(tp)
	}

	if tp == nil || tp.error_ != 0 {
		Con_Print("Out of memory.\n")
	}
}

func TablePrint_AddItem(tp *TablePrint, txt string) {
	if tp == nil || tp.error_ != 0 {
		return
	}

	tp.array = append(tp.array, txt)
	tp.numitems++
}
