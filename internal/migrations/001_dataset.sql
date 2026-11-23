-- +goose Up
CREATE TABLE laureates (
    id text PRIMARY KEY,
    first_name text NOT NULL,
    last_name text NOT NULL,
    gender text NOT NULL,
    born text NOT NULL,
    born_city text NOT NULL,
    born_country text NOT NULL,
    country_code text NOT NULL,
    died text NOT NULL,
    died_city text NOT NULL,
    died_country text NOT NULL,
    died_country_code text NOT NULL
);
CREATE INDEX laureates_country_code_idx ON laureates (country_code, id);

CREATE TABLE prizes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    laureate_id text NOT NULL REFERENCES laureates(id) ON DELETE CASCADE,
    year integer NOT NULL CHECK (year BETWEEN 1901 AND 9999),
    category text NOT NULL,
    share integer NOT NULL CHECK (share > 0),
    motivation text NOT NULL,
    overall_motivation text NOT NULL
);
CREATE INDEX prizes_laureate_id_idx ON prizes (laureate_id, id);

CREATE TABLE affiliations (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    prize_id bigint NOT NULL REFERENCES prizes(id) ON DELETE CASCADE,
    name text NOT NULL,
    city text NOT NULL,
    country text NOT NULL
);
CREATE INDEX affiliations_prize_id_idx ON affiliations (prize_id, id);

CREATE TABLE dataset_metadata (
    id integer PRIMARY KEY CHECK (id = 1),
    last_updated timestamptz
);
INSERT INTO dataset_metadata (id) VALUES (1);

-- +goose Down
DROP TABLE dataset_metadata;
DROP TABLE affiliations;
DROP TABLE prizes;
DROP TABLE laureates;
