# The SOURCE and DESTINATION repo and branch are the same
# It is a hack to ensure we don't really rebase, but we run the hooks
SOURCE_UPSTREAM_REPO="https://github.com/openshift/oadp-operator:oadp-1.6"
DESTINATION_DOWNSTREAM_REPO="openshift/oadp-operator:oadp-1.6"
REBASE_REPO="oadp-rebasebot/oadp-operator:rebase-bot-oadp-1.6"

# Exclude PR#2096 (https://github.com/openshift/oadp-operator/pull/2096)
# which adapted ToSystemAffinity calls to the oadp-dev velero API (2 args).
# On oadp-1.6 velero uses the release-1.18 API which takes a slice instead.
EXTRA_REBASEBOT_ARGS="--always-run-hooks \
  --exclude-commits 6b2eff5a9eff593cfcacb914ab77526dc713d235"
HOOK_SCRIPTS_LOCATION="git:https://github.com/oadp-rebasebot/oadp-rebase/oadp-dev:rebasebot-hook-scripts"
HOOK_SCRIPTS="--post-rebase-hook \
  ${HOOK_SCRIPTS_LOCATION}/go-replace_velero_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/go-mod-tidy-and-commit.sh \
  ${HOOK_SCRIPTS_LOCATION}/oadp-operator-copy-crds-from-velero-and-commit_oadp-1.6.sh \
  ${HOOK_SCRIPTS_LOCATION}/oadp-operator-run-make-bundle-and-commit.sh \
  "
