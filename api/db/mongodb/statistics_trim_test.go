package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/ivpn/dns/api/model"
)

// insertFlat writes one flat document (proxy-statistics-behaviour.md Y19) and returns its _id.
func (s *StatisticsRepositorySuite) insertFlat(coll, profileID, device string, bucketStart time.Time, total int64) primitive.ObjectID {
	id := primitive.NewObjectID()
	_, err := s.client.Database(s.dbName).Collection(coll).InsertOne(context.Background(), bson.D{
		{Key: "_id", Value: id},
		{Key: "bucket_start", Value: bucketStart},
		{Key: "meta", Value: bson.D{{Key: "profile_id", Value: profileID}, {Key: "device_id", Value: device}}},
		{Key: "total", Value: total}, {Key: "blocked", Value: int64(1)}, {Key: "dnssec", Value: int64(0)},
		{Key: "reason_blocklist", Value: int64(1)}, {Key: "proto_doh", Value: total},
	})
	s.Require().NoError(err)
	return id
}

func (s *StatisticsRepositorySuite) docsOf(coll, profileID string) map[primitive.ObjectID]bson.M {
	cur, err := s.client.Database(s.dbName).Collection(coll).Find(context.Background(), bson.D{{Key: "meta.profile_id", Value: profileID}})
	s.Require().NoError(err)
	var docs []bson.M
	s.Require().NoError(cur.All(context.Background(), &docs))
	out := map[primitive.ObjectID]bson.M{}
	for _, d := range docs {
		out[d["_id"].(primitive.ObjectID)] = d
	}
	return out
}

// specRef: api-endpoint-behaviour.md J50 — lowering to 30d deletes the profile's daily documents before
// floor_day(now - 30 days) from the longer daily collections; younger ones stay where they are, and the
// 30d collection, the shorter tiers and other profiles are untouched.
func (s *StatisticsRepositorySuite) TestTrimProfileDailyStatistics() {
	ctx := context.Background()
	now := time.Date(2026, 10, 27, 13, 5, 0, 0, time.UTC)
	since := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC) // floor_day(now - 30 days)

	kept1y := s.insertFlat(statisticsColl1d1y, "p1", "laptop", since, 3)
	s.insertFlat(statisticsColl1d1y, "p1", "laptop", since.Add(-24*time.Hour), 5)
	kept90d := s.insertFlat(statisticsColl1d90d, "p1", "laptop", since.Add(24*time.Hour), 2)
	s.insertFlat(statisticsColl1d90d, "p1", "laptop", since.Add(-60*24*time.Hour), 4)
	in30d := s.insertFlat(statisticsColl1d30d, "p1", "laptop", since.Add(-24*time.Hour), 1)
	hour := s.insertFlat(statisticsColl1h, "p1", "laptop", since.Add(-time.Hour), 1)
	other := s.insertFlat(statisticsColl1d1y, "p2", "laptop", since.Add(-24*time.Hour), 9)

	deleted, err := s.repo.TrimProfileDailyStatistics(ctx, "p1", model.StatisticsRetention30d, now)
	s.Require().NoError(err)
	s.EqualValues(2, deleted)

	s.Equal([]primitive.ObjectID{kept1y}, keys(s.docsOf(statisticsColl1d1y, "p1")), "the document on the cut-off day is kept")
	s.Equal([]primitive.ObjectID{kept90d}, keys(s.docsOf(statisticsColl1d90d, "p1")))
	s.Equal([]primitive.ObjectID{in30d}, keys(s.docsOf(statisticsColl1d30d, "p1")), "the retention's own collection is bounded by its TTL")
	s.Equal([]primitive.ObjectID{hour}, keys(s.docsOf(statisticsColl1h, "p1")))
	s.Equal([]primitive.ObjectID{other}, keys(s.docsOf(statisticsColl1d1y, "p2")))

	again, err := s.repo.TrimProfileDailyStatistics(ctx, "p1", model.StatisticsRetention30d, now)
	s.Require().NoError(err)
	s.Zero(again, "a repeated trim deletes nothing")
}

// specRef: api-endpoint-behaviour.md J50 — 90d trims only the 1y collection; 1y has nothing longer.
func (s *StatisticsRepositorySuite) TestTrimProfileDailyStatistics_OnlyLongerCollections() {
	ctx := context.Background()
	now := time.Date(2026, 10, 27, 13, 5, 0, 0, time.UTC)
	old := now.Add(-200 * 24 * time.Hour).Truncate(24 * time.Hour)
	s.insertFlat(statisticsColl1d1y, "p1", "laptop", old, 1)
	in90d := s.insertFlat(statisticsColl1d90d, "p1", "laptop", old, 1)

	deleted, err := s.repo.TrimProfileDailyStatistics(ctx, "p1", model.StatisticsRetention90d, now)
	s.Require().NoError(err)
	s.EqualValues(1, deleted)
	s.Empty(s.docsOf(statisticsColl1d1y, "p1"))
	s.Equal([]primitive.ObjectID{in90d}, keys(s.docsOf(statisticsColl1d90d, "p1")))

	deleted, err = s.repo.TrimProfileDailyStatistics(ctx, "p1", model.StatisticsRetention1y, now)
	s.Require().NoError(err)
	s.Zero(deleted)
}

func keys(docs map[primitive.ObjectID]bson.M) []primitive.ObjectID {
	out := make([]primitive.ObjectID, 0, len(docs))
	for id := range docs {
		out = append(out, id)
	}
	return out
}

// specRef: api-endpoint-behaviour.md J53 — a history delete is ceiled per tier: the bucket that
// contains the instant goes too; an instant on a bucket start keeps that bucket.
func (s *StatisticsRepositorySuite) TestDeleteProfileStatisticsThrough_CeilsPerTier() {
	d := func(day, h, m int) time.Time { return time.Date(2026, 9, day, h, m, 0, 0, time.UTC) }
	at := d(29, 14, 37)
	for _, start := range []time.Time{d(29, 14, 15), d(29, 14, 30), d(29, 14, 45)} {
		s.insertFlat(statisticsColl15min, "p1", "laptop", start, 1)
	}
	for _, start := range []time.Time{d(29, 13, 0), d(29, 14, 0), d(29, 15, 0)} {
		s.insertFlat(statisticsColl1h, "p1", "laptop", start, 1)
	}
	for _, c := range []string{statisticsColl1d30d, statisticsColl1d90d, statisticsColl1d1y} {
		for _, start := range []time.Time{d(28, 0, 0), d(29, 0, 0), d(30, 0, 0)} {
			s.insertFlat(c, "p1", "laptop", start, 1)
		}
	}
	s.insertFlat(statisticsColl15min, "p2", "laptop", d(29, 14, 15), 1)

	deleted, err := s.repo.DeleteProfileStatisticsThrough(context.Background(), "p1", at)
	s.Require().NoError(err)
	s.EqualValues(10, deleted, "deleted documents are counted across collections")

	s.Equal([]time.Time{d(29, 14, 45)}, s.starts(statisticsColl15min, "p1"))
	s.Equal([]time.Time{d(29, 15, 0)}, s.starts(statisticsColl1h, "p1"))
	for _, c := range []string{statisticsColl1d30d, statisticsColl1d90d, statisticsColl1d1y} {
		s.Equal([]time.Time{d(30, 0, 0)}, s.starts(c, "p1"), c)
	}
	s.EqualValues(1, s.count(statisticsColl15min, "p2"))

	boundary := d(30, 0, 0)
	deleted, err = s.repo.DeleteProfileStatisticsThrough(context.Background(), "p1", boundary)
	s.Require().NoError(err)
	s.EqualValues(2, deleted, "the 15min and 1h documents before the boundary")
	s.Equal([]time.Time{d(30, 0, 0)}, s.starts(statisticsColl1d30d, "p1"), "a bucket starting exactly at the instant is kept")
}
