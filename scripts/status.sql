SELECT
    (SELECT count(*) FROM laureates) AS laureates,
    (SELECT count(*) FROM prizes) AS prizes,
    (SELECT count(*) FROM affiliations) AS affiliations,
    last_updated
FROM dataset_metadata;
