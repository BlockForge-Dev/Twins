use serde::{Deserialize, Serialize};

pub const SOLANA_CHAIN: &str = "solana";
pub const SOLANA_USDC_MINT: &str = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v";

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct StablecoinTransactionEvidence {
    pub chain: String,
    pub signature: String,
    pub slot: u64,
    pub block_time: Option<i64>,
    pub confirmation_status: String,
    pub source_address: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_owner: Option<String>,
    pub destination_address: String,
    pub destination_owner: String,
    pub token: String,
    pub mint: String,
    pub amount: String,
    pub amount_atomic: String,
    pub decimals: u8,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct RejectedTransferEvidence {
    pub chain: String,
    pub signature: String,
    pub slot: u64,
    pub block_time: Option<i64>,
    pub confirmation_status: String,
    pub source_address: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_owner: Option<String>,
    pub destination_address: String,
    pub destination_owner: String,
    pub mint: String,
    pub amount: String,
    pub amount_atomic: String,
    pub decimals: u8,
    pub reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq, Eq)]
pub struct ParseResult {
    pub stablecoin_transactions: Vec<StablecoinTransactionEvidence>,
    pub rejected_transfers: Vec<RejectedTransferEvidence>,
}
