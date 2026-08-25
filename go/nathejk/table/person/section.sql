CREATE TABLE IF NOT EXISTS person_section (
    year VARCHAR(99) NOT NULL,
    slug VARCHAR(99) NOT NULL,
    label VARCHAR(199) NOT NULL DEFAULT "",
    PRIMARY KEY (year, slug)
);
