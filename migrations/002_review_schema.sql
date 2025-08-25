CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    resource_id UUID NOT NULL REFERENCES resources(id),
    reservation_id UUID NOT NULL REFERENCES reservations(id),
    rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment TEXT NOT NULL CHECK (length(comment) <= 1000 AND length(trim(comment)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT reviews_one_per_reservation UNIQUE (reservation_id)
);

-- Indexes for common query patterns
CREATE INDEX idx_reviews_resource_id_created_desc ON reviews (resource_id, created_at DESC, id DESC);
CREATE INDEX idx_reviews_user_id_created_desc ON reviews (user_id, created_at DESC, id DESC);
CREATE INDEX idx_reviews_rating ON reviews (rating);
CREATE INDEX idx_reviews_resource_rating ON reviews (resource_id, rating);

-- Aggregated rating stats per resource
CREATE TABLE resource_rating_stats (
    resource_id UUID PRIMARY KEY REFERENCES resources(id),
    total_reviews INTEGER NOT NULL DEFAULT 0,
    average_rating NUMERIC(3,2) NOT NULL DEFAULT 0.00,
    rating_1_count INTEGER NOT NULL DEFAULT 0,
    rating_2_count INTEGER NOT NULL DEFAULT 0,
    rating_3_count INTEGER NOT NULL DEFAULT 0,
    rating_4_count INTEGER NOT NULL DEFAULT 0,
    rating_5_count INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

