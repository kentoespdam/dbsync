package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

func (m mappingEditorModel) loadData() tea.Msg {
	srcPass, err := crypto.Decrypt(m.conn.SourcePassword, m.masterKey)
	if err != nil {
		return err
	}
	dstPass, err := crypto.Decrypt(m.conn.DestPassword, m.masterKey)
	if err != nil {
		return err
	}

	srcPool, err := mysql.Open(mysql.Config{
		Host: m.conn.SourceHost, Port: m.conn.SourcePort,
		User: m.conn.SourceUser, Password: string(srcPass), DBName: m.conn.SourceDB,
	})
	if err != nil {
		return err
	}
	defer srcPool.Close()

	dstPool, err := mysql.Open(mysql.Config{
		Host: m.conn.DestHost, Port: m.conn.DestPort,
		User: m.conn.DestUser, Password: string(dstPass), DBName: m.conn.DestDB,
	})
	if err != nil {
		return err
	}
	defer dstPool.Close()

	ctx := context.Background()
	srcCols, err := mysql.DescribeColumns(ctx, srcPool.DB(), m.conn.SourceDB, m.tableName)
	if err != nil {
		return err
	}
	dstCols, err := mysql.DescribeColumns(ctx, dstPool.DB(), m.conn.DestDB, m.tableName)
	if err != nil {
		return err
	}

	dbMappings, err := m.store.Mappings().ListByTable(ctx, m.conn.ID, m.tableName)
	if err != nil {
		return err
	}
	mappings := mergeMappings(m.conn.ID, m.tableName, dstCols, srcCols, dbMappings)

	return mappingDataLoadedMsg{
		srcCols:  srcCols,
		dstCols:  dstCols,
		mappings: mappings,
		isNew:    len(dbMappings) == 0,
	}
}

func (m *mappingEditorModel) findDestCol(name string) mysql.Column {
	for _, dc := range m.destCols {
		if dc.Name == name {
			return dc
		}
	}
	return mysql.Column{}
}

func (m *mappingEditorModel) applyFilter() {
	m.filteredMappings = nil
	for _, mp := range m.mappings {
		if m.warningsOnly {
			dc := m.findDestCol(mp.DestColumn)
			icon, _ := m.mappingStatus(mp, dc)
			if icon != "⚠" {
				continue
			}
		}
		if m.filterText != "" && !strings.Contains(strings.ToLower(mp.DestColumn), strings.ToLower(m.filterText)) {
			continue
		}
		m.filteredMappings = append(m.filteredMappings, mp)
	}
	m.refreshTable()
}

func (m *mappingEditorModel) refreshTable() {
	rows := make([]table.Row, len(m.filteredMappings))
	for i, mp := range m.filteredMappings {
		dc := m.findDestCol(mp.DestColumn)
		icon, style := m.mappingStatus(mp, dc)

		src := "-"
		if mp.SourceColumn.Valid {
			src = mp.SourceColumn.String
		}
		def := "-"
		if mp.DefaultValue.Valid {
			def = mp.DefaultValue.String
		}

		rows[i] = table.Row{
			style.Render(icon),
			mp.DestColumn,
			src,
			def,
		}
	}
	m.table.SetRows(rows)
}

func mergeMappings(connID int64, table string, dstCols, srcCols []mysql.Column, existing []storage.Mapping) []storage.Mapping {
	res := make([]storage.Mapping, 0, len(dstCols))
	extMap := make(map[string]storage.Mapping)
	for _, m := range existing {
		extMap[m.DestColumn] = m
	}

	srcMap := make(map[string]bool)
	for _, sc := range srcCols {
		srcMap[sc.Name] = true
	}

	auto := storage.AutoMap(connID, table, srcCols, dstCols)
	autoMap := make(map[string]storage.Mapping)
	for _, m := range auto.Mappings {
		autoMap[m.DestColumn] = m
	}

	for _, dc := range dstCols {
		if m, ok := extMap[dc.Name]; ok {
			res = append(res, m)
		} else if m, ok := autoMap[dc.Name]; ok {
			res = append(res, m)
		}
	}
	return res
}
