use anyhow::{Context, Result, anyhow};
use reqwest::blocking::Client;
use serde::Deserialize;
use serde_json::{Value, json};

#[derive(Debug, Clone, Deserialize)]
pub struct SignatureInfo {
    pub signature: String,
    #[serde(rename = "confirmationStatus")]
    pub confirmation_status: Option<String>,
}

#[derive(Debug, Clone)]
pub struct SolanaRpcClient {
    endpoint: String,
    client: Client,
}

impl SolanaRpcClient {
    pub fn new(endpoint: impl Into<String>) -> Self {
        Self {
            endpoint: endpoint.into(),
            client: Client::new(),
        }
    }

    pub fn get_signatures_for_address(
        &self,
        address: &str,
        limit: usize,
        commitment: &str,
    ) -> Result<Vec<SignatureInfo>> {
        let result = self.call(
            "getSignaturesForAddress",
            json!([
                address,
                {
                    "limit": limit,
                    "commitment": commitment
                }
            ]),
        )?;
        serde_json::from_value(result).context("failed to parse getSignaturesForAddress result")
    }

    pub fn get_transaction(&self, signature: &str, commitment: &str) -> Result<Value> {
        self.call(
            "getTransaction",
            json!([
                signature,
                {
                    "encoding": "jsonParsed",
                    "commitment": commitment,
                    "maxSupportedTransactionVersion": 0
                }
            ]),
        )
    }

    fn call(&self, method: &str, params: Value) -> Result<Value> {
        let response: Value = self
            .client
            .post(&self.endpoint)
            .json(&json!({
                "jsonrpc": "2.0",
                "id": 1,
                "method": method,
                "params": params
            }))
            .send()
            .with_context(|| format!("failed to call Solana RPC method {method}"))?
            .error_for_status()
            .with_context(|| format!("Solana RPC method {method} returned an error status"))?
            .json()
            .with_context(|| format!("failed to decode Solana RPC method {method} response"))?;

        if let Some(error) = response.get("error") {
            return Err(anyhow!("Solana RPC method {method} failed: {error}"));
        }
        response
            .get("result")
            .cloned()
            .with_context(|| format!("Solana RPC method {method} response missing result"))
    }
}
