create table starlight.image
(
    id       serial
        primary key,
    image    varchar not null,
    hash     varchar not null,
    config   json,
    manifest json,
    ready    timestamp with time zone,
    nlayer   integer not null,
    constraint unique_image_hash
        unique (image, hash)
);

alter table starlight.image
    owner to starlight;

create table starlight.layer
(
    id           serial
        primary key,
    size         bigint,
    image        integer not null
        references starlight.image
            on delete cascade,
    "stackIndex" integer,
    layer        integer not null,
    constraint unique_image_stack_index
        unique (image, "stackIndex")
);

alter table starlight.layer
    owner to starlight;

create table starlight.filesystem
(
    id     serial
        primary key,
    digest varchar not null,
    size   bigint,
    ready  timestamp with time zone,
    constraint layer_digest_size
        unique (digest, size)
);

alter table starlight.filesystem
    owner to starlight;


create table starlight.file
(
    id       bigserial
        primary key,
    hash     varchar,
    size     bigint,
    file     varchar,
    "offset" bigint,
    fs       integer
        constraint filesystem_fk
            references starlight.filesystem
            on delete cascade,
    "order"  integer[],
    metadata json,
    constraint unique_file
        unique (file, fs)
);

alter table starlight.file
    owner to starlight;

create index fki_filesystem_fk
    on starlight.file (fs);

create index deduplicate_index
    on starlight.file (hash, size);

create table starlight.tag
(
    name      varchar not null,
    tag       varchar not null,
    platform  varchar not null,
    "imageId" bigint  not null,
    constraint "primary"
        primary key (name, tag, platform)
);

alter table starlight.tag
    owner to starlight;


