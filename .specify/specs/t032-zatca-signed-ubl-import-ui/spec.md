# T032 signed-UBL import UI

The frontend provides authenticated upload, list, detail, review, status,
provenance, validation-error, post, and reject workflows for the durable T031
signed-UBL import lane. It preserves explicit review/pending, failed,
approved, posted, and rejected states and never maps an import to outbound
invoice-issued status.
