package evm

// outpostABI is the minimal ABI for the Outpost contract — only the
// PaymentCreated event is needed by the listener.
const outpostABI = `[
  {
    "anonymous": false,
    "inputs": [
      {"indexed": false, "name": "resource",   "type": "uint8"},
      {"indexed": false, "name": "identity",   "type": "string"},
      {"indexed": false, "name": "streamId",   "type": "string"},
      {"indexed": false, "name": "expiration", "type": "uint256"}
    ],
    "name": "PaymentCreated",
    "type": "event"
  }
]`

// issuerABI is the minimal ABI for ShinzoChallengeIssuerV1 — only the
// view functions used by the scanner are included.
const issuerABI = `[
  {
    "inputs": [{"internalType": "bytes", "name": "headerExtra", "type": "bytes"}],
    "name": "resolveFromExtraData",
    "outputs": [{"internalType": "uint256", "name": "attestationId", "type": "uint256"}],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [{"internalType": "uint256", "name": "attestationId", "type": "uint256"}],
    "name": "attestationCore",
    "outputs": [
      {"internalType": "address",  "name": "withdrawalAddress",  "type": "address"},
      {"internalType": "bytes32",  "name": "delegateKey",        "type": "bytes32"},
      {"internalType": "bytes",    "name": "consensusPubKey",    "type": "bytes"},
      {"internalType": "uint64",   "name": "createdAt",          "type": "uint64"},
      {"internalType": "uint64",   "name": "signatureDeadline",  "type": "uint64"},
      {"internalType": "bool",     "name": "signatureSubmitted", "type": "bool"},
      {"internalType": "uint64",   "name": "signatureSubmittedAt","type": "uint64"},
      {"internalType": "bytes",    "name": "withdrawalSignature","type": "bytes"},
      {"internalType": "bytes15",  "name": "pointer",            "type": "bytes15"},
      {"internalType": "bytes",    "name": "delegateSignature",  "type": "bytes"}
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "DOMAIN_SEPARATOR",
    "outputs": [{"internalType": "bytes32", "name": "", "type": "bytes32"}],
    "stateMutability": "view",
    "type": "function"
  }
]`
