use std::{env, fs};

use anyhow::{Context, Result, bail};
use reqwest::blocking::Client;
use serde::Serialize;
use serde_json::Value;
use twins_solana_watcher::{
    evidence::{ParseResult, SOLANA_USDC_MINT, StablecoinTransactionEvidence},
    parser::{ParseConfig, parse_required_output, parse_transaction},
    rpc::SolanaRpcClient,
};

fn main() -> Result<()> {
    let args: Vec<String> = env::args().collect();
    let Some(command) = args.get(1).map(String::as_str) else {
        usage();
        bail!("missing command");
    };

    match command {
        "verify-fixture" => verify_fixture(&args[2..]),
        "fetch-signature" => fetch_signature(&args[2..]),
        "scan-wallet" => scan_wallet(&args[2..]),
        _ => {
            usage();
            bail!("unknown command: {command}");
        }
    }
}

fn verify_fixture(args: &[String]) -> Result<()> {
    let input = required_flag(args, "--input")?;
    let config = parse_config(args)?;
    let value: Value = serde_json::from_str(
        &fs::read_to_string(&input).with_context(|| format!("failed to read {input}"))?,
    )
    .with_context(|| format!("failed to parse {input} as JSON"))?;

    let result = parse_required_output(parse_transaction(&value, &config)?)?;
    maybe_post(args, &result.stablecoin_transactions)?;
    print_json(&result)
}

fn fetch_signature(args: &[String]) -> Result<()> {
    let rpc_url = required_flag(args, "--rpc-url")?;
    let signature = required_flag(args, "--signature")?;
    let config = parse_config(args)?;
    let rpc = SolanaRpcClient::new(rpc_url);

    let value = rpc.get_transaction(&signature, &config.confirmation_status)?;
    let result = parse_required_output(parse_transaction(&value, &config)?)?;
    maybe_post(args, &result.stablecoin_transactions)?;
    print_json(&result)
}

fn scan_wallet(args: &[String]) -> Result<()> {
    let rpc_url = required_flag(args, "--rpc-url")?;
    let wallet = required_flag(args, "--wallet")?;
    let commitment = optional_flag(args, "--commitment").unwrap_or_else(|| "finalized".to_string());
    let limit = optional_flag(args, "--limit")
        .unwrap_or_else(|| "20".to_string())
        .parse::<usize>()
        .context("--limit must be a number")?;
    let usdc_mint =
        optional_flag(args, "--usdc-mint").unwrap_or_else(|| SOLANA_USDC_MINT.to_string());
    let rpc = SolanaRpcClient::new(rpc_url);

    let signatures = rpc.get_signatures_for_address(&wallet, limit, &commitment)?;
    let mut combined = ParseResult::default();
    for signature in signatures {
        let confirmation_status = signature
            .confirmation_status
            .clone()
            .unwrap_or_else(|| commitment.clone());
        let config = ParseConfig {
            watched_wallet: wallet.clone(),
            usdc_mint: usdc_mint.clone(),
            confirmation_status,
        };
        let value = rpc.get_transaction(&signature.signature, &commitment)?;
        let result = parse_transaction(&value, &config)
            .with_context(|| format!("failed to parse {}", signature.signature))?;
        combined
            .stablecoin_transactions
            .extend(result.stablecoin_transactions);
        combined
            .rejected_transfers
            .extend(result.rejected_transfers);
    }

    maybe_post(args, &combined.stablecoin_transactions)?;
    print_json(&combined)
}

fn parse_config(args: &[String]) -> Result<ParseConfig> {
    Ok(ParseConfig {
        watched_wallet: required_flag(args, "--wallet")?,
        usdc_mint: optional_flag(args, "--usdc-mint")
            .unwrap_or_else(|| SOLANA_USDC_MINT.to_string()),
        confirmation_status: optional_flag(args, "--commitment")
            .unwrap_or_else(|| "finalized".to_string()),
    })
}

fn maybe_post(args: &[String], events: &[StablecoinTransactionEvidence]) -> Result<()> {
    let Some(post_url) = optional_flag(args, "--post-url") else {
        return Ok(());
    };
    let api_key = required_flag(args, "--api-key")?;
    let client = Client::new();

    for event in events {
        client
            .post(&post_url)
            .bearer_auth(&api_key)
            .json(event)
            .send()
            .with_context(|| format!("failed to post evidence for {}", event.signature))?
            .error_for_status()
            .with_context(|| format!("Twins API rejected evidence for {}", event.signature))?;
    }

    Ok(())
}

fn required_flag(args: &[String], name: &str) -> Result<String> {
    optional_flag(args, name).with_context(|| format!("{name} is required"))
}

fn optional_flag(args: &[String], name: &str) -> Option<String> {
    args.windows(2)
        .find(|window| window[0] == name)
        .map(|window| window[1].clone())
}

fn print_json<T: Serialize>(value: &T) -> Result<()> {
    println!("{}", serde_json::to_string_pretty(value)?);
    Ok(())
}

fn usage() {
    eprintln!(
        "Usage:\n  twins-solana-watcher verify-fixture --input <json> --wallet <wallet> [--commitment finalized] [--post-url <url> --api-key <key>]\n  twins-solana-watcher fetch-signature --rpc-url <url> --signature <sig> --wallet <wallet> [--commitment finalized] [--post-url <url> --api-key <key>]\n  twins-solana-watcher scan-wallet --rpc-url <url> --wallet <wallet> [--limit 20] [--commitment finalized] [--post-url <url> --api-key <key>]"
    );
}
