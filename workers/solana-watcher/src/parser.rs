use std::collections::HashMap;

use anyhow::{Context, Result, bail};
use serde_json::Value;

use crate::evidence::{
    ParseResult, RejectedTransferEvidence, SOLANA_CHAIN, StablecoinTransactionEvidence,
};

#[derive(Debug, Clone)]
pub struct ParseConfig {
    pub watched_wallet: String,
    pub usdc_mint: String,
    pub confirmation_status: String,
}

#[derive(Debug, Clone)]
struct TokenAccountMeta {
    owner: Option<String>,
    mint: Option<String>,
    decimals: Option<u8>,
}

#[derive(Debug, Clone)]
struct ParsedTransfer {
    source_address: String,
    source_owner: Option<String>,
    destination_address: String,
    destination_owner: Option<String>,
    mint: Option<String>,
    amount_atomic: String,
    amount: String,
    decimals: u8,
}

pub fn parse_transaction(value: &Value, config: &ParseConfig) -> Result<ParseResult> {
    let tx = value
        .get("result")
        .filter(|result| !result.is_null())
        .unwrap_or(value);
    let signature = tx
        .pointer("/transaction/signatures/0")
        .and_then(Value::as_str)
        .context("transaction signature missing")?
        .to_string();
    let slot = tx
        .get("slot")
        .and_then(Value::as_u64)
        .context("transaction slot missing")?;
    let block_time = tx.get("blockTime").and_then(Value::as_i64);

    let account_keys = account_keys(tx);
    let token_accounts = token_account_metadata(tx, &account_keys);

    let mut result = ParseResult::default();
    for instruction in parsed_instructions(tx) {
        let Some(transfer) = parse_transfer_instruction(instruction, &token_accounts) else {
            continue;
        };
        let destination_owner = transfer
            .destination_owner
            .clone()
            .unwrap_or_else(|| transfer.destination_address.clone());
        let is_inbound = transfer.destination_address == config.watched_wallet
            || destination_owner == config.watched_wallet;
        if !is_inbound {
            continue;
        }

        let mint = transfer.mint.clone().unwrap_or_default();
        if mint == config.usdc_mint && transfer.decimals == 6 {
            result
                .stablecoin_transactions
                .push(StablecoinTransactionEvidence {
                    chain: SOLANA_CHAIN.to_string(),
                    signature: signature.clone(),
                    slot,
                    block_time,
                    confirmation_status: config.confirmation_status.clone(),
                    source_address: transfer.source_address,
                    source_owner: transfer.source_owner,
                    destination_address: transfer.destination_address,
                    destination_owner,
                    token: "USDC".to_string(),
                    mint,
                    amount: transfer.amount,
                    amount_atomic: transfer.amount_atomic,
                    decimals: transfer.decimals,
                });
        } else {
            result.rejected_transfers.push(RejectedTransferEvidence {
                chain: SOLANA_CHAIN.to_string(),
                signature: signature.clone(),
                slot,
                block_time,
                confirmation_status: config.confirmation_status.clone(),
                source_address: transfer.source_address,
                source_owner: transfer.source_owner,
                destination_address: transfer.destination_address,
                destination_owner,
                mint,
                amount: transfer.amount,
                amount_atomic: transfer.amount_atomic,
                decimals: transfer.decimals,
                reason: "wrong_token".to_string(),
            });
        }
    }

    Ok(result)
}

fn account_keys(tx: &Value) -> Vec<String> {
    tx.pointer("/transaction/message/accountKeys")
        .and_then(Value::as_array)
        .map(|keys| {
            keys.iter()
                .filter_map(|key| {
                    key.as_str().map(str::to_string).or_else(|| {
                        key.get("pubkey")
                            .and_then(Value::as_str)
                            .map(str::to_string)
                    })
                })
                .collect()
        })
        .unwrap_or_default()
}

fn token_account_metadata(
    tx: &Value,
    account_keys: &[String],
) -> HashMap<String, TokenAccountMeta> {
    let mut metadata = HashMap::new();
    for balance_path in ["/meta/preTokenBalances", "/meta/postTokenBalances"] {
        let Some(balances) = tx.pointer(balance_path).and_then(Value::as_array) else {
            continue;
        };
        for balance in balances {
            let Some(account_index) = balance.get("accountIndex").and_then(Value::as_u64) else {
                continue;
            };
            let Some(account) = account_keys.get(account_index as usize) else {
                continue;
            };
            metadata.insert(
                account.clone(),
                TokenAccountMeta {
                    owner: balance
                        .get("owner")
                        .and_then(Value::as_str)
                        .map(str::to_string),
                    mint: balance
                        .get("mint")
                        .and_then(Value::as_str)
                        .map(str::to_string),
                    decimals: balance
                        .pointer("/uiTokenAmount/decimals")
                        .and_then(Value::as_u64)
                        .and_then(|decimals| u8::try_from(decimals).ok()),
                },
            );
        }
    }
    metadata
}

fn parsed_instructions(tx: &Value) -> Vec<&Value> {
    let mut instructions = Vec::new();
    if let Some(top_level) = tx
        .pointer("/transaction/message/instructions")
        .and_then(Value::as_array)
    {
        instructions.extend(top_level.iter());
    }
    if let Some(inner_groups) = tx
        .pointer("/meta/innerInstructions")
        .and_then(Value::as_array)
    {
        for group in inner_groups {
            if let Some(inner) = group.get("instructions").and_then(Value::as_array) {
                instructions.extend(inner.iter());
            }
        }
    }
    instructions
}

fn parse_transfer_instruction(
    instruction: &Value,
    token_accounts: &HashMap<String, TokenAccountMeta>,
) -> Option<ParsedTransfer> {
    let program = instruction.get("program").and_then(Value::as_str);
    let program_id = instruction.get("programId").and_then(Value::as_str);
    let is_spl_token = program == Some("spl-token")
        || program_id == Some("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA");
    if !is_spl_token {
        return None;
    }

    let parsed = instruction.get("parsed")?;
    let parsed_type = parsed.get("type").and_then(Value::as_str)?;
    if parsed_type != "transfer" && parsed_type != "transferChecked" {
        return None;
    }

    let info = parsed.get("info")?;
    let source_address = info.get("source").and_then(Value::as_str)?.to_string();
    let destination_address = info.get("destination").and_then(Value::as_str)?.to_string();
    let source_meta = token_accounts.get(&source_address);
    let destination_meta = token_accounts.get(&destination_address);

    let mint = info
        .get("mint")
        .and_then(Value::as_str)
        .map(str::to_string)
        .or_else(|| destination_meta.and_then(|meta| meta.mint.clone()))
        .or_else(|| source_meta.and_then(|meta| meta.mint.clone()));

    let (amount_atomic, decimals, amount) = if let Some(token_amount) = info.get("tokenAmount") {
        let raw = token_amount
            .get("amount")
            .and_then(Value::as_str)?
            .to_string();
        let decimals = token_amount
            .get("decimals")
            .and_then(Value::as_u64)
            .and_then(|value| u8::try_from(value).ok())?;
        let amount = token_amount
            .get("uiAmountString")
            .and_then(Value::as_str)
            .map(str::to_string)
            .unwrap_or_else(|| format_atomic_amount(&raw, decimals));
        (raw, decimals, amount)
    } else {
        let raw = info.get("amount").and_then(Value::as_str)?.to_string();
        let decimals = destination_meta
            .and_then(|meta| meta.decimals)
            .or_else(|| source_meta.and_then(|meta| meta.decimals))?;
        let amount = format_atomic_amount(&raw, decimals);
        (raw, decimals, amount)
    };

    Some(ParsedTransfer {
        source_address,
        source_owner: source_meta.and_then(|meta| meta.owner.clone()),
        destination_address,
        destination_owner: destination_meta.and_then(|meta| meta.owner.clone()),
        mint,
        amount_atomic,
        amount,
        decimals,
    })
}

fn format_atomic_amount(raw: &str, decimals: u8) -> String {
    if decimals == 0 {
        return raw.to_string();
    }

    let decimals = decimals as usize;
    let mut digits = raw.trim_start_matches('0').to_string();
    if digits.is_empty() {
        digits.push('0');
    }
    if digits.len() <= decimals {
        let padding = "0".repeat(decimals + 1 - digits.len());
        digits = format!("{padding}{digits}");
    }

    let split_at = digits.len() - decimals;
    let whole = &digits[..split_at];
    let mut fraction = digits[split_at..].trim_end_matches('0').to_string();
    if fraction.is_empty() {
        fraction.push('0');
    }
    format!("{whole}.{fraction}")
}

pub fn parse_required_output(result: ParseResult) -> Result<ParseResult> {
    if result.stablecoin_transactions.is_empty() && result.rejected_transfers.is_empty() {
        bail!("no inbound stablecoin transfer evidence found for watched wallet");
    }
    Ok(result)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::evidence::SOLANA_USDC_MINT;

    #[test]
    fn extracts_inbound_usdc_transfer_for_registered_wallet() {
        let value: Value =
            serde_json::from_str(include_str!("../fixtures/inbound_usdc_transfer.json")).unwrap();

        let result = parse_transaction(
            &value,
            &ParseConfig {
                watched_wallet: "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH".to_string(),
                usdc_mint: SOLANA_USDC_MINT.to_string(),
                confirmation_status: "finalized".to_string(),
            },
        )
        .unwrap();

        assert_eq!(result.stablecoin_transactions.len(), 1);
        assert!(result.rejected_transfers.is_empty());

        let evidence = &result.stablecoin_transactions[0];
        assert_eq!(evidence.token, "USDC");
        assert_eq!(evidence.amount_atomic, "500000000");
        assert_eq!(evidence.amount, "500");
        assert_eq!(
            evidence.destination_owner,
            "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH"
        );
    }

    #[test]
    fn rejects_wrong_token_for_registered_wallet() {
        let value: Value =
            serde_json::from_str(include_str!("../fixtures/wrong_token_transfer.json")).unwrap();

        let result = parse_transaction(
            &value,
            &ParseConfig {
                watched_wallet: "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH".to_string(),
                usdc_mint: SOLANA_USDC_MINT.to_string(),
                confirmation_status: "finalized".to_string(),
            },
        )
        .unwrap();

        assert!(result.stablecoin_transactions.is_empty());
        assert_eq!(result.rejected_transfers.len(), 1);
        assert_eq!(result.rejected_transfers[0].reason, "wrong_token");
    }
}
