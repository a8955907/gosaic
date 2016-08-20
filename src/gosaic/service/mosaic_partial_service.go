package service

import (
	"errors"
	"gosaic/model"
	"sync"

	"gopkg.in/gorp.v1"
)

type MosaicPartialService interface {
	Service
	Get(int64) (*model.MosaicPartial, error)
	Insert(*model.MosaicPartial) error
	Count(*model.Mosaic) (int64, error)
	CountMissing(*model.Mosaic) (int64, error)
	GetMissing(*model.Mosaic) *model.MacroPartial
	GetRandomMissing(*model.Mosaic) *model.MacroPartial
	FindAllPartialViews(*model.Mosaic, string, int, int) ([]*model.MosaicPartialView, error)
}

type mosaicPartialServiceImpl struct {
	dbMap *gorp.DbMap
	m     sync.Mutex
}

func NewMosaicPartialService(dbMap *gorp.DbMap) MosaicPartialService {
	return &mosaicPartialServiceImpl{dbMap: dbMap}
}

func (s *mosaicPartialServiceImpl) DbMap() *gorp.DbMap {
	return s.dbMap
}

func (s *mosaicPartialServiceImpl) Register() error {
	s.DbMap().AddTableWithName(model.MosaicPartial{}, "mosaic_partials").SetKeys(true, "id")
	return nil
}

func (s *mosaicPartialServiceImpl) Get(id int64) (*model.MosaicPartial, error) {
	c, err := s.DbMap().Get(model.MosaicPartial{}, id)
	if err != nil {
		return nil, err
	}

	if c == nil {
		return nil, nil
	}

	m, ok := c.(*model.MosaicPartial)
	if !ok {
		return nil, errors.New("Unable to type cast mosaic partial")
	}

	if m.Id == int64(0) {
		return nil, nil
	}

	return m, nil
}

func (s *mosaicPartialServiceImpl) Insert(mosaicPartial *model.MosaicPartial) error {
	return s.DbMap().Insert(mosaicPartial)
}

func (s *mosaicPartialServiceImpl) CountMissing(mosaic *model.Mosaic) (int64, error) {
	s.m.Lock()
	defer s.m.Unlock()

	sql := `
		select count(*)
		from macro_partials map
		where map.macro_id = ?
		and not exists (
			select 1 from mosaic_partials mop
			where mop.mosaic_id = ?
			and mop.macro_partial_id = map.id
		)
	`
	return s.DbMap().SelectInt(sql, mosaic.MacroId, mosaic.Id)
}

func (s *mosaicPartialServiceImpl) Count(mosaic *model.Mosaic) (int64, error) {
	s.m.Lock()
	defer s.m.Unlock()

	sql := `
		select count(*)
		from mosaic_partials
		where mosaic_partials.mosaic_id = ?
	`
	return s.DbMap().SelectInt(sql, mosaic.Id)
}

func (s *mosaicPartialServiceImpl) GetMissing(mosaic *model.Mosaic) *model.MacroPartial {
	s.m.Lock()
	defer s.m.Unlock()

	sql := `
		select *
		from macro_partials map
		where map.macro_id = ?
		and not exists (
			select 1 from mosaic_partials mop
			where mop.mosaic_id = ?
			and mop.macro_partial_id = map.id
		)
		order by map.id asc
		limit 1
	`
	var macroPartial model.MacroPartial
	err := s.DbMap().SelectOne(&macroPartial, sql, mosaic.MacroId, mosaic.Id)
	if err != nil {
		return nil
	}

	return &macroPartial
}

func (s *mosaicPartialServiceImpl) GetRandomMissing(mosaic *model.Mosaic) *model.MacroPartial {
	s.m.Lock()
	defer s.m.Unlock()

	sql := `
		select *
		from macro_partials map
		where map.macro_id = ?
		and not exists (
			select 1 from mosaic_partials mop
			where mop.mosaic_id = ?
			and mop.macro_partial_id = map.id
		)
		order by random()
		limit 1
	`
	var macroPartial model.MacroPartial
	err := s.DbMap().SelectOne(&macroPartial, sql, mosaic.MacroId, mosaic.Id)
	if err != nil {
		return nil
	}

	return &macroPartial
}

func (s *mosaicPartialServiceImpl) FindAllPartialViews(mosaic *model.Mosaic, order string, limit, offset int) ([]*model.MosaicPartialView, error) {
	s.m.Lock()
	defer s.m.Unlock()

	sql := `
		select mosaic_partials.id as mosaic_partial_id,
			gidx.id as gidx_id,
			gidx.aspect_id as gidx_aspect_id,
			gidx.path as gidx_path,
			gidx.md5sum as gidx_md5sum,
			gidx.width as gidx_width,
			gidx.height as gidx_height,
			gidx.orientation as gidx_orientation,
			cover_partials.id as cover_partial_id,
			cover_partials.cover_id as cover_partial_cover_id,
			cover_partials.aspect_id as cover_partial_aspect_id,
			cover_partials.x1 as cover_partial_x1,
			cover_partials.y1 as cover_partial_y1,
			cover_partials.x2 as cover_partial_x2,
			cover_partials.y2 as cover_partial_y2
		from mosaic_partials
		inner join gidx_partials
			on mosaic_partials.gidx_partial_id = gidx_partials.id
		inner join gidx
			on gidx_partials.gidx_id = gidx.id
		inner join macro_partials
			on mosaic_partials.macro_partial_id = macro_partials.id
		inner join cover_partials
			on macro_partials.cover_partial_id = cover_partials.id
		where mosaic_partials.mosaic_id = ?
		order by ?
		limit ?
		offset ?
	`

	var mosaicPartialViews []*model.MosaicPartialView
	rows, err := s.dbMap.Db.Query(sql, mosaic.Id, order, limit, offset)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var r model.MosaicPartialView = model.MosaicPartialView{
			Gidx:         &model.Gidx{},
			CoverPartial: &model.CoverPartial{},
		}

		err = rows.Scan(
			&r.MosaicPartialId,
			&r.Gidx.Id,
			&r.Gidx.AspectId,
			&r.Gidx.Path,
			&r.Gidx.Md5sum,
			&r.Gidx.Width,
			&r.Gidx.Height,
			&r.Gidx.Orientation,
			&r.CoverPartial.Id,
			&r.CoverPartial.CoverId,
			&r.CoverPartial.AspectId,
			&r.CoverPartial.X1,
			&r.CoverPartial.Y1,
			&r.CoverPartial.X2,
			&r.CoverPartial.Y2,
		)
		if err != nil {
			return nil, err
		}
		mosaicPartialViews = append(mosaicPartialViews, &r)
	}

	return mosaicPartialViews, nil
}