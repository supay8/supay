package ports

import "context"

// FiscalServiceProvider resolves a tenant's capabilities without exposing SDK clients.
type FiscalServiceProvider interface {
	GetForCompany(context.Context, string) (FiscalService, error)
	Invalidate(string)
}

type FiscalIdentity interface{ CodigoSistema() string }

type FiscalCredentials interface {
	RequestCUIS(context.Context, CredentialRequest) (CuisResult, error)
	RequestCUFD(context.Context, CredentialRequest) (CufdResult, error)
}

type FiscalSynchronizer interface {
	FiscalCredentials
	Synchronize(context.Context, FiscalSyncRequest, FiscalSyncOperation) (FiscalSyncResult, error)
}

type FiscalSingle interface {
	Emit(context.Context, FiscalDocument) (FiscalResult, error)
	VerifyStatus(context.Context, FiscalDocumentQuery) (FiscalDocumentResult, error)
	Annul(context.Context, FiscalDocumentQuery, int) (FiscalDocumentResult, error)
	RevertAnnul(context.Context, FiscalDocumentQuery) (FiscalDocumentResult, error)
}

type FiscalEvents interface {
	RegisterSignificantEvent(context.Context, FiscalEvent) (FiscalEventResult, error)
}

type FiscalBatch interface {
	SendPackage(context.Context, FiscalPackage) (FiscalPackageResult, error)
	ValidatePackage(context.Context, FiscalPackage, string) (FiscalPackageResult, error)
	SendBulk(context.Context, FiscalBulk) (FiscalPackageResult, error)
	ValidateBulk(context.Context, FiscalBulk, string) (FiscalPackageResult, error)
}

type FiscalSigner interface {
	SignXML(context.Context, FiscalSignRequest) (FiscalSignResult, error)
}

// SerializedDocument is XML before signing; CUF is derived from this exact document.
type SerializedDocument struct {
	XML          []byte
	CUF          string
	DocumentType int
}

type DocumentSerializer interface {
	Serialize(context.Context, FiscalDocument, int) (SerializedDocument, error)
}

type PackedLot struct {
	Archive string
	Hash    string
}

// LotPacker deterministically packs exact XML bytes; it never signs or performs I/O.
type LotPacker interface {
	PackDocument([]byte) (PackedLot, error)
	PackLot([][]byte) (PackedLot, error)
}

// FiscalEmissionPipeline separates preparation from network dispatch. Prepared bytes
// must be durably stored before Dispatch, and must never be rebuilt during dispatch.
type FiscalEmissionPipeline interface {
	PrepareEmission(context.Context, FiscalDocument) (FiscalResult, error)
	DispatchEmission(context.Context, FiscalDocument, FiscalResult) (FiscalResult, error)
}
