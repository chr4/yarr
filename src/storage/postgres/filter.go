package postgres

import (
	"database/sql"
	"log"

	"github.com/nkanaev/yarr/src/storage/model"
)

func (s *PostgresStorage) ListFilters() []model.Filter {
	result := make([]model.Filter, 0)
	rows, err := s.db.Query(`
		select
			f.id, f.pattern, f.match_content, f.match_page, f.match_link, f.case_sensitive, f.feed_id, f.enabled,
			(select count(*) from items i where i.filter_id = f.id and i.status != $1),
			case when f.match_page then (
				select count(*) from item_pages p join items i on i.id = p.item_id
				where i.feed_id = f.feed_id and p.blocked
			) else 0 end
		from filters f
		order by f.id
	`, model.STARRED)
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var f model.Filter
		err = rows.Scan(
			&f.Id, &f.Pattern, &f.MatchContent, &f.MatchPage, &f.MatchLink, &f.CaseSensitive, &f.FeedId, &f.Enabled,
			&f.Hidden, &f.Blocked,
		)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

func (s *PostgresStorage) CreateFilter(params model.FilterParams) *model.Filter {
	var id int64
	err := s.db.QueryRow(`
		insert into filters (pattern, match_content, match_page, match_link, case_sensitive, feed_id, enabled)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id`,
		params.Pattern,
		params.MatchContent,
		params.MatchPage,
		params.MatchLink,
		params.CaseSensitive,
		params.FeedId,
		params.Enabled,
	).Scan(&id)
	if err != nil {
		log.Print(err)
		return nil
	}
	return &model.Filter{
		Id:            id,
		Pattern:       params.Pattern,
		MatchContent:  params.MatchContent,
		MatchPage:     params.MatchPage,
		MatchLink:     params.MatchLink,
		CaseSensitive: params.CaseSensitive,
		FeedId:        params.FeedId,
		Enabled:       params.Enabled,
	}
}

func (s *PostgresStorage) UpdateFilter(id int64, params model.FilterParams) bool {
	_, err := s.db.Exec(`
		update filters set
			pattern        = $2,
			match_content  = $3,
			match_page     = $4,
			match_link     = $5,
			case_sensitive = $6,
			feed_id        = $7,
			enabled        = $8
		where id = $1`,
		id,
		params.Pattern,
		params.MatchContent,
		params.MatchPage,
		params.MatchLink,
		params.CaseSensitive,
		params.FeedId,
		params.Enabled,
	)
	if err != nil {
		log.Print(err)
	}
	return err == nil
}

func (s *PostgresStorage) DeleteFilter(id int64) bool {
	_, err := s.db.Exec(`delete from filters where id = $1`, id)
	if err != nil {
		log.Print(err)
	}
	return err == nil
}

func (s *PostgresStorage) ForEachItem(withContent, withPage bool, fn func(item model.Item)) error {
	content := "''"
	if withContent {
		content = "i.content"
	}
	page := "null"
	if withPage {
		page = "p.content"
	}
	rows, err := s.db.Query(`
		select i.id, i.feed_id, i.title, i.link, i.date, i.status, i.filter_id, ` + content + `, ` + page + `
		from items i
		left join item_pages p on p.item_id = i.id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var x model.Item
		err = rows.Scan(&x.Id, &x.FeedId, &x.Title, &x.Link, &x.Date, &x.Status, &x.FilterId, &x.Content, &x.Page)
		if err != nil {
			return err
		}
		fn(x)
	}
	return rows.Err()
}

func (s *PostgresStorage) ListItemGUIDs(feedId int64) map[string]bool {
	result := make(map[string]bool)
	rows, err := s.db.Query(`select guid from items where feed_id = $1`, feedId)
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var guid string
		if err := rows.Scan(&guid); err != nil {
			log.Print(err)
			return result
		}
		result[guid] = true
	}
	return result
}

// ListPageItems returns the newest items of a feed, for fetching their
// article pages: all of them, or only those without page yet.
func (s *PostgresStorage) ListPageItems(feedId int64, limit int, onlyMissing bool) []model.Item {
	result := make([]model.Item, 0)
	rows, err := s.db.Query(`
		select i.id, i.feed_id, i.link
		from (
			select id, feed_id, link, date
			from items
			where feed_id = $1
			order by date desc, id desc
			limit $2
		) i
		where not $3 or not exists (select 1 from item_pages p where p.item_id = i.id)
		order by i.date desc, i.id desc
	`, feedId, limit, onlyMissing)
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var x model.Item
		if err := rows.Scan(&x.Id, &x.FeedId, &x.Link); err != nil {
			log.Print(err)
			return result
		}
		result = append(result, x)
	}
	return result
}

func (s *PostgresStorage) SetItemPages(pages map[int64]model.ItemPage) bool {
	tx, err := s.db.Begin()
	if err != nil {
		log.Print(err)
		return false
	}
	defer tx.Rollback()

	for id, page := range pages {
		if err := setItemPage(tx, id, page); err != nil {
			log.Print(err)
			return false
		}
	}
	if err := tx.Commit(); err != nil {
		log.Print(err)
		return false
	}
	return true
}

func setItemPage(tx *sql.Tx, itemId int64, page model.ItemPage) error {
	_, err := tx.Exec(`
		insert into item_pages (item_id, content, blocked) values ($1, $2, $3)
		on conflict (item_id) do update set content = excluded.content, blocked = excluded.blocked`,
		itemId, page.Content, page.Blocked,
	)
	return err
}

func (s *PostgresStorage) CountBlockedPages(feedId int64) int {
	var count int
	err := s.db.QueryRow(`
		select count(*) from item_pages p join items i on i.id = p.item_id
		where i.feed_id = $1 and p.blocked`,
		feedId,
	).Scan(&count)
	if err != nil {
		log.Print(err)
	}
	return count
}

func (s *PostgresStorage) SetItemFilters(filterIds map[int64]*int64) bool {
	tx, err := s.db.Begin()
	if err != nil {
		log.Print(err)
		return false
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`update items set filter_id = $2 where id = $1`)
	if err != nil {
		log.Print(err)
		return false
	}
	defer stmt.Close()

	for id, filterId := range filterIds {
		if _, err := stmt.Exec(id, filterId); err != nil {
			log.Print(err)
			return false
		}
	}
	if err := tx.Commit(); err != nil {
		log.Print(err)
		return false
	}
	return true
}
