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

// specRef: api-endpoint-behaviour.md J50 — lowering to 30d copies the documents younger than the
// new retention unchanged into statistics_1d_30d and leaves none of the profile's in the longer
// collections; older documents are dropped; other profiles are untouched.
func (s *StatisticsRepositorySuite) TestMoveProfileDailyStatistics_ToShorterRetention() {
	ctx := context.Background()
	day := time.Now().UTC().Truncate(24 * time.Hour)
	since := day.Add(-30 * 24 * time.Hour)

	recent1y := s.insertFlat(statisticsColl1d1y, "p1", "laptop", day.Add(-2*24*time.Hour), 7)
	edge1y := s.insertFlat(statisticsColl1d1y, "p1", "phone", since, 3)
	s.insertFlat(statisticsColl1d1y, "p1", "laptop", since.Add(-24*time.Hour), 5)
	recent90d := s.insertFlat(statisticsColl1d90d, "p1", "laptop", day.Add(-24*time.Hour), 2)
	s.insertFlat(statisticsColl1d90d, "p1", "laptop", day.Add(-60*24*time.Hour), 4)
	existing30d := s.insertFlat(statisticsColl1d30d, "p1", "laptop", day, 1)
	other := s.insertFlat(statisticsColl1d1y, "p2", "laptop", day.Add(-2*24*time.Hour), 9)
	before1y := s.docsOf(statisticsColl1d1y, "p1")

	moved, err := s.repo.MoveProfileDailyStatistics(ctx, "p1", model.StatisticsRetention30d, since)
	s.Require().NoError(err)
	s.Equal(3, moved)

	s.Empty(s.docsOf(statisticsColl1d1y, "p1"))
	s.Empty(s.docsOf(statisticsColl1d90d, "p1"))
	got := s.docsOf(statisticsColl1d30d, "p1")
	s.Len(got, 4)
	for _, id := range []primitive.ObjectID{recent1y, edge1y} {
		s.Equal(before1y[id], got[id], "copied unchanged")
	}
	s.Contains(got, recent90d)
	s.Contains(got, existing30d)
	s.Contains(s.docsOf(statisticsColl1d1y, "p2"), other)
}

// specRef: api-endpoint-behaviour.md J50 — lowering 1y to 90d moves into statistics_1d_90d and
// keeps statistics_1d_30d as it is (a shorter collection is never touched).
func (s *StatisticsRepositorySuite) TestMoveProfileDailyStatistics_To90dLeavesShorterCollection() {
	ctx := context.Background()
	day := time.Now().UTC().Truncate(24 * time.Hour)
	since := day.Add(-90 * 24 * time.Hour)
	in30d := s.insertFlat(statisticsColl1d30d, "p1", "laptop", day.Add(-10*24*time.Hour), 1)
	in1y := s.insertFlat(statisticsColl1d1y, "p1", "laptop", day.Add(-50*24*time.Hour), 2)
	s.insertFlat(statisticsColl1d1y, "p1", "laptop", day.Add(-200*24*time.Hour), 3)

	moved, err := s.repo.MoveProfileDailyStatistics(ctx, "p1", model.StatisticsRetention90d, since)
	s.Require().NoError(err)
	s.Equal(1, moved)
	s.Contains(s.docsOf(statisticsColl1d30d, "p1"), in30d)
	s.Len(s.docsOf(statisticsColl1d90d, "p1"), 1)
	s.Contains(s.docsOf(statisticsColl1d90d, "p1"), in1y)
	s.Empty(s.docsOf(statisticsColl1d1y, "p1"))
}

// specRef: api-endpoint-behaviour.md J50 — more documents than one batch, and nothing to move.
func (s *StatisticsRepositorySuite) TestMoveProfileDailyStatistics_BatchesAndNoop() {
	ctx := context.Background()
	day := time.Now().UTC().Truncate(24 * time.Hour)
	since := day.Add(-30 * 24 * time.Hour)
	for i := range 2500 {
		s.insertFlat(statisticsColl1d1y, "p1", "d"+string(rune('a'+i%26)), day.Add(-time.Duration(i%20)*24*time.Hour), 1)
	}
	moved, err := s.repo.MoveProfileDailyStatistics(ctx, "p1", model.StatisticsRetention30d, since)
	s.Require().NoError(err)
	s.Equal(2500, moved)
	s.Empty(s.docsOf(statisticsColl1d1y, "p1"))
	s.EqualValues(2500, s.count(statisticsColl1d30d, "p1"))

	moved, err = s.repo.MoveProfileDailyStatistics(ctx, "p1", model.StatisticsRetention30d, since)
	s.Require().NoError(err)
	s.Zero(moved)

	moved, err = s.repo.MoveProfileDailyStatistics(ctx, "p1", model.StatisticsRetention1y, since)
	s.Require().NoError(err)
	s.Zero(moved, "nothing is longer than 1y")
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

	s.Require().NoError(s.repo.DeleteProfileStatisticsThrough(context.Background(), "p1", at))

	s.Equal([]time.Time{d(29, 14, 45)}, s.starts(statisticsColl15min, "p1"))
	s.Equal([]time.Time{d(29, 15, 0)}, s.starts(statisticsColl1h, "p1"))
	for _, c := range []string{statisticsColl1d30d, statisticsColl1d90d, statisticsColl1d1y} {
		s.Equal([]time.Time{d(30, 0, 0)}, s.starts(c, "p1"), c)
	}
	s.EqualValues(1, s.count(statisticsColl15min, "p2"))

	boundary := d(30, 0, 0)
	s.Require().NoError(s.repo.DeleteProfileStatisticsThrough(context.Background(), "p1", boundary))
	s.Equal([]time.Time{d(30, 0, 0)}, s.starts(statisticsColl1d30d, "p1"), "a bucket starting exactly at the instant is kept")
}
