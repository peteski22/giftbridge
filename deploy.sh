#!/usr/bin/env bash
#
# GiftBridge Deployment Script
# ============================
# Deploys GiftBridge to AWS Lambda with all required infrastructure.
#
# Usage:
#   ./deploy.sh [options]
#
# Options:
#   --stack-name NAME    CloudFormation stack name (default: giftbridge)
#   --region REGION      AWS region (default: from AWS CLI config)
#   --skip-download      Skip downloading binary, use local build
#   --help               Show this help message
#

set -euo pipefail

# Colors for output.
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color.

# Default values.
STACK_NAME="giftbridge"
REGION=""
SKIP_DOWNLOAD=false
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/infrastructure/.env"
TEMPLATE_FILE="${SCRIPT_DIR}/infrastructure/sam/template.yaml"
BINARY_NAME="bootstrap"
REPO="peteski22/giftbridge"

# -----------------------------------------------------------------------------
# Helper functions
# -----------------------------------------------------------------------------

info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1" >&2
}

fatal() {
    error "$1"
    exit 1
}

show_help() {
    sed -n '3,16p' "$0" | sed 's/^# //' | sed 's/^#//'
    exit 0
}

# -----------------------------------------------------------------------------
# Prerequisite checks
# -----------------------------------------------------------------------------

check_aws_cli() {
    info "Checking for AWS CLI..."
    if ! command -v aws &> /dev/null; then
        fatal "AWS CLI is not installed. Please install it from: https://aws.amazon.com/cli/"
    fi
    success "AWS CLI found: $(aws --version | head -1)"
}

check_aws_auth() {
    info "Checking AWS authentication..."
    if ! aws sts get-caller-identity &> /dev/null; then
        fatal "Not authenticated with AWS. Run 'aws configure' or set AWS credentials."
    fi
    local account_id
    account_id=$(aws sts get-caller-identity --query Account --output text)
    success "Authenticated to AWS account: ${account_id}"
}

check_env_file() {
    info "Checking for configuration file..."
    if [[ ! -f "${ENV_FILE}" ]]; then
        error "Configuration file not found: ${ENV_FILE}"
        echo ""
        echo "To create it, run:"
        echo "  cp infrastructure/.env.example infrastructure/.env"
        echo ""
        echo "Then edit infrastructure/.env with your credentials."
        exit 1
    fi
    success "Configuration file found"
}

validate_env_vars() {
    info "Validating configuration..."
    local missing=()

    # Required variables.
    local required_vars=(
        "BLACKBAUD_CLIENT_ID"
        "BLACKBAUD_CLIENT_SECRET"
        "BLACKBAUD_ENVIRONMENT_ID"
        "BLACKBAUD_REFRESH_TOKEN"
        "BLACKBAUD_SUBSCRIPTION_KEY"
        "FUNDRAISEUP_API_KEY"
        "GIFT_FUND_ID"
    )

    for var in "${required_vars[@]}"; do
        if [[ -z "${!var:-}" ]]; then
            missing+=("${var}")
        fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
        error "Missing required configuration:"
        for var in "${missing[@]}"; do
            echo "  - ${var}"
        done
        echo ""
        echo "Please set these values in: ${ENV_FILE}"
        exit 1
    fi

    success "Configuration valid"
}

# -----------------------------------------------------------------------------
# Binary management
# -----------------------------------------------------------------------------

get_latest_release() {
    curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

download_binary() {
    info "Fetching latest release..."
    local version
    version=$(get_latest_release)

    if [[ -z "${version}" ]]; then
        warn "No releases found. Will build locally."
        return 1
    fi

    info "Downloading ${version}..."
    local url="https://github.com/${REPO}/releases/download/${version}/bootstrap.zip"

    if ! curl -sL "${url}" -o "${SCRIPT_DIR}/bootstrap.zip"; then
        warn "Download failed. Will build locally."
        return 1
    fi

    info "Extracting binary..."
    unzip -o -q "${SCRIPT_DIR}/bootstrap.zip" -d "${SCRIPT_DIR}"
    rm "${SCRIPT_DIR}/bootstrap.zip"

    success "Downloaded ${version}"
    return 0
}

build_binary() {
    info "Building binary locally..."

    if ! command -v go &> /dev/null; then
        fatal "Go is not installed and no pre-built binary available. Please install Go from: https://go.dev/dl/"
    fi

    GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o "${SCRIPT_DIR}/${BINARY_NAME}" "${SCRIPT_DIR}/cmd/sync"
    success "Binary built successfully"
}

ensure_binary() {
    if [[ "${SKIP_DOWNLOAD}" == "true" ]]; then
        build_binary
        return
    fi

    # Try to download, fall back to local build.
    if ! download_binary; then
        build_binary
    fi

    if [[ ! -f "${SCRIPT_DIR}/${BINARY_NAME}" ]]; then
        fatal "Binary not found after build/download"
    fi
}

# -----------------------------------------------------------------------------
# Deployment
# -----------------------------------------------------------------------------

create_deployment_bucket() {
    local bucket_name="${STACK_NAME}-deployment-${AWS_ACCOUNT_ID}"

    info "Checking deployment bucket..."
    if ! aws s3api head-bucket --bucket "${bucket_name}" 2>/dev/null; then
        info "Creating deployment bucket: ${bucket_name}"
        if [[ -n "${REGION}" ]]; then
            aws s3api create-bucket \
                --bucket "${bucket_name}" \
                --region "${REGION}" \
                --create-bucket-configuration LocationConstraint="${REGION}" \
                > /dev/null
        else
            aws s3api create-bucket --bucket "${bucket_name}" > /dev/null
        fi
    fi
    success "Deployment bucket ready: ${bucket_name}"
    echo "${bucket_name}"
}

deploy_stack() {
    local bucket_name="$1"

    info "Packaging template..."
    local packaged_template="${SCRIPT_DIR}/.packaged-template.yaml"

    aws cloudformation package \
        --template-file "${TEMPLATE_FILE}" \
        --s3-bucket "${bucket_name}" \
        --output-template-file "${packaged_template}" \
        > /dev/null

    info "Deploying stack: ${STACK_NAME}"

    local region_arg=""
    if [[ -n "${REGION}" ]]; then
        region_arg="--region ${REGION}"
    fi

    # shellcheck disable=SC2086
    aws cloudformation deploy \
        --template-file "${packaged_template}" \
        --stack-name "${STACK_NAME}" \
        --capabilities CAPABILITY_IAM \
        ${region_arg} \
        --parameter-overrides \
            "BlackbaudClientId=${BLACKBAUD_CLIENT_ID}" \
            "BlackbaudClientSecret=${BLACKBAUD_CLIENT_SECRET}" \
            "BlackbaudEnvironmentId=${BLACKBAUD_ENVIRONMENT_ID}" \
            "BlackbaudRefreshToken=${BLACKBAUD_REFRESH_TOKEN}" \
            "BlackbaudSubscriptionKey=${BLACKBAUD_SUBSCRIPTION_KEY}" \
            "FundraiseUpApiKey=${FUNDRAISEUP_API_KEY}" \
            "GiftFundId=${GIFT_FUND_ID}" \
            "GiftCampaignId=${GIFT_CAMPAIGN_ID:-}" \
            "GiftAppealId=${GIFT_APPEAL_ID:-}" \
            "GiftType=${GIFT_TYPE:-Donation}" \
            "ScheduleExpression=${SCHEDULE_EXPRESSION:-rate(1 hour)}"

    rm -f "${packaged_template}"
    success "Deployment complete!"
}

show_outputs() {
    echo ""
    info "Stack outputs:"
    aws cloudformation describe-stacks \
        --stack-name "${STACK_NAME}" \
        --query 'Stacks[0].Outputs[*].[OutputKey,OutputValue]' \
        --output table
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------

main() {
    # Parse arguments.
    while [[ $# -gt 0 ]]; do
        case $1 in
            --stack-name)
                STACK_NAME="$2"
                shift 2
                ;;
            --region)
                REGION="$2"
                shift 2
                ;;
            --skip-download)
                SKIP_DOWNLOAD=true
                shift
                ;;
            --help|-h)
                show_help
                ;;
            *)
                fatal "Unknown option: $1"
                ;;
        esac
    done

    echo ""
    echo "╔═══════════════════════════════════════╗"
    echo "║       GiftBridge Deployment           ║"
    echo "╚═══════════════════════════════════════╝"
    echo ""

    # Prerequisite checks.
    check_aws_cli
    check_aws_auth
    check_env_file

    # Load environment.
    info "Loading configuration..."
    set -a
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    set +a
    success "Configuration loaded"

    validate_env_vars

    # Get AWS account ID for bucket naming.
    AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

    # Ensure we have a binary.
    ensure_binary

    # Deploy.
    echo ""
    info "Starting deployment..."
    local bucket_name
    bucket_name=$(create_deployment_bucket)
    deploy_stack "${bucket_name}"

    show_outputs

    echo ""
    success "GiftBridge is now running!"
    echo ""
    echo "Next steps:"
    echo "  - Check CloudWatch Logs for sync activity"
    echo "  - Verify donations are appearing in Raiser's Edge"
    echo ""
}

main "$@"
