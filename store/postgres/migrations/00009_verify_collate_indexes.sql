-- +goose Up
-- The verify merge-diff pages both identity streams in byte order using
-- COLLATE "C" comparisons; without matching indexes every page is a full
-- table scan plus a top-N sort.
CREATE INDEX patients_id_collate_c ON patients (id COLLATE "C");
CREATE INDEX clinics_id_collate_c ON clinics (id COLLATE "C");
CREATE INDEX clinicians_id_collate_c ON clinicians (id COLLATE "C");
CREATE INDEX patient_deletions_id_collate_c ON patient_deletions (id COLLATE "C");
CREATE INDEX clinician_deletions_id_collate_c ON clinician_deletions (id COLLATE "C");
CREATE INDEX clinic_deletions_id_collate_c ON clinic_deletions (id COLLATE "C");
CREATE INDEX merge_plans_id_collate_c ON merge_plans (id COLLATE "C");
CREATE INDEX redox_messages_id_collate_c ON redox_messages (id COLLATE "C");
CREATE INDEX scheduled_summary_reports_orders_id_collate_c ON scheduled_summary_reports_orders (id COLLATE "C");
CREATE INDEX xealth_preorders_id_collate_c ON xealth_preorders (id COLLATE "C");
CREATE INDEX xealth_orders_id_collate_c ON xealth_orders (id COLLATE "C");
CREATE INDEX xealth_report_views_id_collate_c ON xealth_report_views (id COLLATE "C");
CREATE INDEX migrations_user_id_collate_c ON migrations (user_id COLLATE "C");

-- +goose Down
DROP INDEX patients_id_collate_c;
DROP INDEX clinics_id_collate_c;
DROP INDEX clinicians_id_collate_c;
DROP INDEX patient_deletions_id_collate_c;
DROP INDEX clinician_deletions_id_collate_c;
DROP INDEX clinic_deletions_id_collate_c;
DROP INDEX merge_plans_id_collate_c;
DROP INDEX redox_messages_id_collate_c;
DROP INDEX scheduled_summary_reports_orders_id_collate_c;
DROP INDEX xealth_preorders_id_collate_c;
DROP INDEX xealth_orders_id_collate_c;
DROP INDEX xealth_report_views_id_collate_c;
DROP INDEX migrations_user_id_collate_c;
