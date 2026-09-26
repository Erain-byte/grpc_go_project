class ServiceError(Exception):
    """Base class for errors that are safe to expose to an RPC caller."""

    default_message = "service request failed"

    def __init__(self, public_message: str | None = None) -> None:
        self.public_message = public_message or self.default_message
        super().__init__(self.public_message)


class InvalidArgumentError(ServiceError):
    default_message = "invalid request argument"


class ResourceNotFoundError(ServiceError):
    default_message = "resource was not found"


class ResourceAlreadyExistsError(ServiceError):
    default_message = "resource already exists"


class UnauthenticatedError(ServiceError):
    default_message = "authentication is required"


class PermissionDeniedError(ServiceError):
    default_message = "permission denied"


class FailedPreconditionError(ServiceError):
    default_message = "request precondition failed"


class ConflictError(ServiceError):
    default_message = "request conflicted with the current state"


class ResourceExhaustedError(ServiceError):
    default_message = "resource limit was exceeded"


class ModelTimeoutError(ServiceError):
    default_message = "model request timed out"


class ServiceUnavailableError(ServiceError):
    default_message = "service is temporarily unavailable"


class DataLossError(ServiceError):
    default_message = "data was lost or corrupted"
