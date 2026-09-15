---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: The Siphon reconciler
---

`internal/controller/siphon` reconciles the `apps.gitlab.com/v2alpha1` `Siphon` resource, the change
data capture pipeline from the PostgreSQL database of a GitLab instance into ClickHouse. For the
design of the resource, see [ADR 27](adr/0027-siphon-custom-resource.md).

The resource references its PostgreSQL source, its NATS server and its ClickHouse sink. It
provisions none of them. Create the prerequisites below first, or the pipeline runs and replicates
nothing.

## Enable the reconciler

The reconciler is gated twice, like [the GitLabCore reconciler](gitlabcore.md):

- The `bridge` build tag compiles it in. Without the tag, `siphon_stub.go` takes its place.
- `ENABLE_BRIDGE=true` registers it at runtime. The `Siphon` definition ships in no release, and a
  watch on a definition the cluster does not serve fails the manager on start.

Run `task install_v2alpha1_crds` to install the definitions and grant the manager ServiceAccount
what the reconciler needs.

Run `task retrieve-charts` to fetch the Siphon chart at the version in `SIPHON_CHART_VERSIONS` into
`charts/`. A version the Operator does not carry is a configuration error, not a download.

The Siphon application image tag is pinned in `internal/controller/siphon/values.go`
(`siphonImageTag`), not derived. The chart declares no `appVersion` and defaults `image.tag` to
null, and the published tags are unrelated to the chart version, so there is nothing to resolve it
from offline. Set `spec.chart.values.image.tag` to run another build. Resolving it from the release
manifest is [issue 2208](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/2208).

## Create the prerequisites

### PostgreSQL

Run the source server with `wal_level = logical`, and point `spec.source.host` at the primary. A
logical replication slot is not created on a standby.

Upstream documents this setup under
[Following the least privilege model](https://gitlab.com/gitlab-org/analytics-section/siphon#following-the-least-privilege-model).

The same DDL ships as runnable files in the
[`database` directory](https://gitlab.com/gitlab-org/analytics-section/siphon/-/tree/main/database),
where `setup_siphon_source_db.sh` applies them and `99-remove-siphon.sql` reverses them. The steps
below differ from those files in two ways.

Run the DDL as the GitLab application owner, so that one role owns the function, the publication and
the tables. `ALTER PUBLICATION ... ADD TABLE` requires the role that runs it to own the publication
and the table, and `SECURITY DEFINER` makes that role the owner of the function. Upstream instead
installs the function as a superuser, which bypasses the ownership check entirely.

Do not copy the `ALTER PUBLICATION ... OWNER TO siphon` line from upstream
`3-create_siphon_publication.sql`. It has no effect under a superuser-owned function, and under an
application-owned one it is what causes `must be owner of publication` on every `ADD TABLE`, while
the workload still reports Running.

Name the publication `siphon_producer_main`, the producer application identifier. Upstream names it
`siphon_publication_main_1` and only requires a match with `replication.publication_name`, but the
Operator derives that setting from the fixed topology. A name that does not match fails loudly, not
silently. Siphon does not find the configured publication, tries `CREATE PUBLICATION`, and the
`siphon` role has no `CREATE` on the database, so the producer reports
`permission denied for database`.

```sql
CREATE OR REPLACE FUNCTION siphon_alter_publication(pbl TEXT, tbl TEXT, op INTEGER)
RETURNS void AS $$
DECLARE
  operation TEXT;
BEGIN
  IF pbl !~ '^[a-zA-Z_][a-zA-Z0-9_]*$' THEN
    RAISE EXCEPTION 'Invalid publication name';
  END IF;
  IF tbl !~ '^[a-zA-Z_][a-zA-Z0-9_]*\.[a-zA-Z_][a-zA-Z0-9_]*$' THEN
    RAISE EXCEPTION 'Invalid table name format: must be schema-qualified';
  END IF;
  IF op = 0 THEN
    operation := 'ADD';
  ELSIF op = 1 THEN
    operation := 'DROP';
  ELSE
    RAISE EXCEPTION 'Invalid operation parameter: op must be 0 (ADD) or 1 (DROP)';
  END IF;
  EXECUTE pg_catalog.format('ALTER PUBLICATION %s %s TABLE %s', pbl, operation, tbl);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

REVOKE EXECUTE ON FUNCTION siphon_alter_publication(TEXT, TEXT, INTEGER) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION siphon_alter_publication(TEXT, TEXT, INTEGER) TO siphon;

CREATE PUBLICATION siphon_producer_main;

GRANT SELECT ON ALL TABLES IN SCHEMA public TO siphon;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO siphon;
```

Without `FOR ROLE`, `ALTER DEFAULT PRIVILEGES` applies to the tables that the role which runs it
creates later. That is the other reason to run this DDL as the application owner.

The producer also snapshots the partition schemas directly, so grant those after the GitLab
migrations have created them. PostgreSQL cannot grant on a schema that does not exist yet.

```sql
GRANT USAGE ON SCHEMA gitlab_partitions_dynamic, gitlab_partitions_static TO siphon;
GRANT SELECT ON ALL TABLES IN SCHEMA gitlab_partitions_dynamic, gitlab_partitions_static TO siphon;

ALTER DEFAULT PRIVILEGES IN SCHEMA gitlab_partitions_dynamic GRANT SELECT ON TABLES TO siphon;
ALTER DEFAULT PRIVILEGES IN SCHEMA gitlab_partitions_static GRANT SELECT ON TABLES TO siphon;
```

Default privileges matter more in these schemas than in `public`. GitLab creates partitions on a
schedule, and `GRANT ... ON ALL TABLES` covers only the partitions that exist when it runs.

The `siphon` role needs `REPLICATION` and nothing more. It owns neither the tables nor the
publication, which is what the function is for.

> [!note]
> Check this DDL against the upstream files before you rely on it. This page last matched them in
> September 2026.

### NATS

Enable JetStream. Siphon uses streams, a key-value bucket to elect the active consumer, and an
object store for events too large for a stream message.

### ClickHouse

Run the GitLab ClickHouse migrations first. They create the target tables; Siphon does not. Point
`spec.sink` at the database that holds them, over the native protocol on port 9000. Port 8123 is the
HTTP interface, which Siphon does not speak, and the API rejects it.

`spec.sink.database` and `spec.sink.username` are required rather than defaulted. The Operator uses
the reference it is given instead of guessing the schema layout of the instance.

### Secrets

The chart creates no Secret. Each credential reaches the pod as an environment variable and is
substituted into the configuration at startup, so create the Secrets first and reference them from
`spec.source.passwordSecretRef`, `spec.sink.passwordSecretRef` and `spec.queue.auth`.

## Deploy a pipeline

Apply a `GitLabCore`, wait for it to report `Available`, then apply a `Siphon`. See
[the sample](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/config/samples/siphon_v2alpha1.yaml).

The reconciler waits for the referenced instance. Nothing in `GitLabCore` readiness may depend on a
`Siphon`, or the two deadlock.

Name the resource with at most 31 characters. The Operator derives the Deployment, ServiceAccount and
pod selector names from it, and the longest of those is
`<name>-siphon-reconciler-clickhouse-sa`, which has to fit the 63 characters a label value allows.

## Table definitions

Each pod generates its configuration at startup from table definitions that ship as an OCI image
tagged with the GitLab application version. That version is read from `status.gitlabVersion` of the
referenced `GitLabCore`, which is the only place it is published: the chart version is an unrelated
number, and the chart an instance renders may be one the Operator does not carry on disk, because it
can be pulled at reconcile time.

A `GitLabCore` that is available but has not published the version yet is a wait
(`Initialized=False`, reason `GitLabVersionPending`), not a failure, so the Siphon converges on its
own once the instance reconciles.

`spec.tables.source` selects how they reach the pods:

| Value | Behavior |
|---|---|
| `Auto` | The default. Resolves to `ConfigMap`, which works everywhere. |
| `ImageVolume` | Mounts the image. Needs a cluster and a container runtime that serve native OCI image volumes. |
| `ConfigMap` | The Operator pulls the image, extracts the definitions, and publishes them to a ConfigMap it owns. |

Set `ImageVolume` explicitly on a cluster that does serve image volumes. `Auto` does not select it,
because the Kubernetes version is only one of the two preconditions: the container runtime has to
serve them too (containerd 2.0 or CRI-O 1.31 and later), and nothing in the Kubernetes API reports
that. A 1.35 cluster on a runtime without it admits the pod, pulls the image, and then fails to
create the container with `invalid mount config for type "bind": field Source must not be empty`,
which names neither the field nor the runtime.

On the `ConfigMap` path the Operator reaches the registry from the reconcile loop. Reference a
`kubernetes.io/dockerconfigjson` Secret from `spec.tables.pullSecretRef` for a private mirror. The
digest, not the tag, decides whether a fetch is needed, so a tag that moves is picked up, and the
registry is asked at most once every five minutes.

The extracted definitions have to fit one ConfigMap, which cannot exceed 1 MiB. The set is about
17 KiB today. Over the budget, `TablesResolved` reports `TablesTooLarge` and names the two ways out.

## Verify that data is flowing

`Available` reports that the producer and the consumer have rolled out. Their readiness probes return
503 until each holds its lock, so it does prove they are connected and leading.

It does not prove that change data is flowing. The producer reports ready against a publication with
no tables in it, so a pipeline that replicates nothing can report `Running`. Check the source server
directly:

```sql
SELECT count(*) FROM pg_publication_tables WHERE pubname = 'siphon_producer_main';
SELECT slot_name, active, wal_status FROM pg_replication_slots;
```

The count must be above zero and the slot must be active. A count of zero with a running workload
means the publication is not owned by the owner of `siphon_alter_publication`, so every `ADD TABLE`
through the function fails.

The reconciler excludes the reconciler Deployment from `Available`. Siphon serves placeholder health
endpoints for that role which answer 200 unconditionally, so its readiness proves only that a process
is listening.

Check the sink through the tables the application reads, not through the `siphon_*` table of the same
name. Several of those are `ENGINE = Null` in the GitLab ClickHouse schema, `siphon_issues` and
`siphon_merge_requests` among them: they are insert entry points for materialized views that write
into `work_items` and `merge_requests`, and the Null engine discards its own copy. So

```sql
SELECT count() FROM siphon_issues;      -- always 0, however well the pipeline is running
SELECT count() FROM work_items;         -- what actually arrived
```

Which name is real and which is an entry point is a property of the GitLab schema rather than of
Siphon, so read `system.tables.engine` before trusting a count.

### Availability flaps while the initial snapshot runs

The producer pauses replication to merge each snapshotted table, and its readiness probe answers 503
while it does, so the pod flaps ready and not ready for the length of the initial snapshot and
`Available` follows it back to `False` after first reaching `True`. On an instance with almost no
data the snapshot took about 10 seconds per table, so around 15 minutes for the roughly 100 tables
of a GitLab schema.

Nothing is wrong when this happens, but anything that waits for `Running` and then asserts on it is
flaky, so give it a settling window. Whether the probe should report ready through a merge is a
question for the Siphon team, and whether the producer belongs in `Available` during a snapshot is
one for this reconciler.

## Delete a pipeline

Deleting a `Siphon` removes what the chart rendered and the ConfigMap the Operator published. It does
not remove the replication slot, the publication or the NATS stream, and it cannot: the Operator did
not create them.

PostgreSQL retains write-ahead log for an inactive slot indefinitely, so a forgotten slot fills the
volume of the source server. The reconciler emits a `ReplicationSlotRetained` warning event on
deletion, and `status.replicationSlot`, `status.publication` and `status.streamName` carry the names.
Drop them by hand:

```sql
SELECT pg_drop_replication_slot('siphon_producer_main_slot');
DROP PUBLICATION siphon_producer_main;
```

## Change the topology

The topology is fixed: one producer, one ClickHouse consumer, one reconciler, one stream. Replicas
buy active and standby failover, never parallelism, because the producer holds a PostgreSQL advisory
lock and the consumer a NATS key-value lock.

To shard, replace `siphonLayoutConfigMap.data` through `spec.chart.values` with more application
identifiers. Use underscores: the producer identifier becomes the publication name and, with a
`_slot` suffix, the replication slot name, and a hyphen is a syntax error in a slot name. The
examples the chart ships use hyphens, and its `values-full.yaml` also writes `sslmode` where the
producer reads `ssl_mode`, which it ignores silently.

Changing the identifier set reassigns tables across shards. For a producer that means a new
publication and a new replication slot, so treat it as a migration rather than a tuning change.

## End-to-end tests

`task e2e-suite SUITE=siphon` deploys the Operator as an image and drives a real `Siphon` through
the API server. It needs no GitLab. The reconciler takes the finalizer and records the topology
before it resolves `gitlabRef`, so the schema, the wait reasons, that nothing renders early, and the
warning that deletion retains the replication slot are all observable against an empty cluster.

Nothing past `resolveGitLab` is covered yet. Both tiers that would cover it need a fixture the
harness does not provide:

- What a `Siphon` renders needs a `GitLabCore` reporting `Available`. That means every workload the
  GitLab chart renders has ready replicas. Nothing can short-circuit the status, because the same
  Operator reconciles the instance and overwrites what a test writes.
- Whether a row arrives needs a PostgreSQL primary that carries the publication, the
  `siphon_alter_publication` function and the grants, NATS with JetStream, and a ClickHouse that
  holds the target tables. The Operator creates none of them, and the migrations of the instance
  create the target tables.

Assert the data path on the table the application reads, not on the `siphon_` table of the same
name. Several of those tables use `ENGINE = Null`, among them `siphon_issues` and
`siphon_merge_requests`. They are insert entry points for materialized views that write into
`work_items` and `merge_requests`. A count on them returns 0 however well the pipeline runs, so read
`system.tables.engine` first. `siphon_events` is a real `ReplacingMergeTree`.

For more information, see [Tests](testing.md).
