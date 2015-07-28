package storable

import (
	"errors"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	NonNewDocumentErr  = errors.New("Cannot insert a non new document.")
	NewDocumentErr     = errors.New("Cannot updated a new document.")
	EmptyQueryInRawErr = errors.New("Empty queries are not allowed on raw ops.")
	EmptyIdErr         = errors.New("A document without id is not allowed.")
)

type Store struct {
	db         *mgo.Database
	collection string
}

// NewStore returns a new Store instance
func NewStore(db *mgo.Database, collection string) *Store {
	return &Store{
		db:         db,
		collection: collection,
	}
}

// Insert insert the given document in the collection, returns error if no-new
// document is given. The document id is setted if is empty.
func (s *Store) Insert(doc DocumentBase) error {
	if !doc.IsNew() {
		return NonNewDocumentErr
	}

	if len(doc.GetId()) == 0 {
		doc.SetId(bson.NewObjectId())
	}

	sess, c := s.getSessionAndCollection()
	defer sess.Close()

	err := c.Insert(doc)
	if err == nil {
		doc.SetIsNew(false)
	}

	return err
}

// Update update the given document in the collection, returns error if a new
// document is given.
func (s *Store) Update(doc DocumentBase) error {
	if doc.IsNew() {
		return NewDocumentErr
	}

	sess, c := s.getSessionAndCollection()
	defer sess.Close()

	return c.UpdateId(doc.GetId(), doc)
}

// Save insert or update the given document in the collection, a document with
// id should be provided. Upsert is used (http://godoc.org/gopkg.in/mgo.v2#Collection.Upsert)
func (s *Store) Save(doc DocumentBase) (updated bool, err error) {
	id := doc.GetId()
	if len(id) == 0 {
		return false, EmptyIdErr
	}

	sess, c := s.getSessionAndCollection()
	defer sess.Close()

	inf, err := c.UpsertId(id, doc)
	if err != nil {
		return false, err
	}

	doc.SetIsNew(false)
	return inf.Updated > 0, nil
}

// Delete remove the document from the collection
func (s *Store) Delete(doc DocumentBase) error {
	sess, c := s.getSessionAndCollection()
	defer sess.Close()

	return c.RemoveId(doc.GetId())
}

// Find executes the given query in the collection
func (s *Store) Find(q Query) (*ResultSet, error) {
	sess, c := s.getSessionAndCollection()
	mq := c.Find(q.GetCriteria())

	if !q.GetSort().IsEmpty() {
		mq.Sort(q.GetSort().String())
	}

	if q.GetSkip() != 0 {
		mq.Skip(q.GetSkip())
	}

	if q.GetLimit() != 0 {
		mq.Limit(q.GetLimit())
	}

	return &ResultSet{session: sess, mgoQuery: mq}, nil
}

// RawUpdate performes a direct update in the collection, update is wrapped on
// a $set operator. If a query without criteria is given EmptyQueryInRawErr is
// returned
func (s *Store) RawUpdate(query Query, update interface{}, multi bool) error {
	criteria := query.GetCriteria()
	if len(criteria) == 0 {
		return EmptyQueryInRawErr
	}

	sess, c := s.getSessionAndCollection()
	defer sess.Close()

	var err error
	if multi {
		_, err = c.UpdateAll(criteria, bson.M{"$set": update})
	} else {
		err = c.Update(criteria, bson.M{"$set": update})
	}

	return err
}

// RawDelete performes a direct remove in the collection. If a query without
// criteria is given EmptyQueryInRawErr is returned
func (s *Store) RawDelete(query Query, multi bool) error {
	criteria := query.GetCriteria()
	if len(criteria) == 0 {
		return EmptyQueryInRawErr
	}

	sess, c := s.getSessionAndCollection()
	defer sess.Close()

	var err error
	if multi {
		_, err = c.RemoveAll(criteria)
	} else {
		err = c.Remove(criteria)
	}

	return err
}

func (s *Store) getSessionAndCollection() (*mgo.Session, *mgo.Collection) {
	sess := s.db.Session.Clone()

	return sess, sess.DB(s.db.Name).C(s.collection)
}
