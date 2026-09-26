import datetime

from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ChatRole(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CHAT_ROLE_UNSPECIFIED: _ClassVar[ChatRole]
    CHAT_ROLE_SYSTEM: _ClassVar[ChatRole]
    CHAT_ROLE_USER: _ClassVar[ChatRole]
    CHAT_ROLE_ASSISTANT: _ClassVar[ChatRole]
    CHAT_ROLE_TOOL: _ClassVar[ChatRole]
CHAT_ROLE_UNSPECIFIED: ChatRole
CHAT_ROLE_SYSTEM: ChatRole
CHAT_ROLE_USER: ChatRole
CHAT_ROLE_ASSISTANT: ChatRole
CHAT_ROLE_TOOL: ChatRole

class ChatRequest(_message.Message):
    __slots__ = ("message", "conversation_id", "user_id", "model", "timestamp")
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    message: str
    conversation_id: str
    user_id: str
    model: str
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, message: _Optional[str] = ..., conversation_id: _Optional[str] = ..., user_id: _Optional[str] = ..., model: _Optional[str] = ..., timestamp: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ChatResponse(_message.Message):
    __slots__ = ("reply", "conversation_id", "user_id", "model", "timestamp", "usage")
    REPLY_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    reply: str
    conversation_id: str
    user_id: str
    model: str
    timestamp: _timestamp_pb2.Timestamp
    usage: TokenUsage
    def __init__(self, reply: _Optional[str] = ..., conversation_id: _Optional[str] = ..., user_id: _Optional[str] = ..., model: _Optional[str] = ..., timestamp: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., usage: _Optional[_Union[TokenUsage, _Mapping]] = ...) -> None: ...

class StreamChatRequest(_message.Message):
    __slots__ = ("message", "conversation_id", "user_id", "model", "timestamp")
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    message: str
    conversation_id: str
    user_id: str
    model: str
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, message: _Optional[str] = ..., conversation_id: _Optional[str] = ..., user_id: _Optional[str] = ..., model: _Optional[str] = ..., timestamp: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class StreamChatResponse(_message.Message):
    __slots__ = ("delta", "conversation_id", "model", "finished", "finish_reason", "usage")
    DELTA_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    FINISHED_FIELD_NUMBER: _ClassVar[int]
    FINISH_REASON_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    delta: str
    conversation_id: str
    model: str
    finished: bool
    finish_reason: str
    usage: TokenUsage
    def __init__(self, delta: _Optional[str] = ..., conversation_id: _Optional[str] = ..., model: _Optional[str] = ..., finished: _Optional[bool] = ..., finish_reason: _Optional[str] = ..., usage: _Optional[_Union[TokenUsage, _Mapping]] = ...) -> None: ...

class TokenUsage(_message.Message):
    __slots__ = ("prompt_tokens", "completion_tokens", "total_tokens")
    PROMPT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COMPLETION_TOKENS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_TOKENS_FIELD_NUMBER: _ClassVar[int]
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int
    def __init__(self, prompt_tokens: _Optional[int] = ..., completion_tokens: _Optional[int] = ..., total_tokens: _Optional[int] = ...) -> None: ...

class GetChatHistoryRequest(_message.Message):
    __slots__ = ("conversation_id", "user_id", "limit", "offset")
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    user_id: str
    limit: int
    offset: int
    def __init__(self, conversation_id: _Optional[str] = ..., user_id: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class GetChatHistoryResponse(_message.Message):
    __slots__ = ("messages", "total_count")
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_COUNT_FIELD_NUMBER: _ClassVar[int]
    messages: _containers.RepeatedCompositeFieldContainer[ChatMessage]
    total_count: int
    def __init__(self, messages: _Optional[_Iterable[_Union[ChatMessage, _Mapping]]] = ..., total_count: _Optional[int] = ...) -> None: ...

class ChatMessage(_message.Message):
    __slots__ = ("id", "role", "content", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    role: ChatRole
    content: str
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., role: _Optional[_Union[ChatRole, str]] = ..., content: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetChatListRequest(_message.Message):
    __slots__ = ("user_id", "limit", "offset")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    limit: int
    offset: int
    def __init__(self, user_id: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class GetChatListResponse(_message.Message):
    __slots__ = ("conversations", "total_count")
    CONVERSATIONS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_COUNT_FIELD_NUMBER: _ClassVar[int]
    conversations: _containers.RepeatedCompositeFieldContainer[Conversation]
    total_count: int
    def __init__(self, conversations: _Optional[_Iterable[_Union[Conversation, _Mapping]]] = ..., total_count: _Optional[int] = ...) -> None: ...

class Conversation(_message.Message):
    __slots__ = ("conversation_id", "user_id", "model", "last_message_time", "message_count")
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    LAST_MESSAGE_TIME_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_COUNT_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    user_id: str
    model: str
    last_message_time: _timestamp_pb2.Timestamp
    message_count: int
    def __init__(self, conversation_id: _Optional[str] = ..., user_id: _Optional[str] = ..., model: _Optional[str] = ..., last_message_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., message_count: _Optional[int] = ...) -> None: ...

class CallModelRequest(_message.Message):
    __slots__ = ("model", "input", "parameters")
    MODEL_FIELD_NUMBER: _ClassVar[int]
    INPUT_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    model: str
    input: str
    parameters: _struct_pb2.Struct
    def __init__(self, model: _Optional[str] = ..., input: _Optional[str] = ..., parameters: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class CallModelResponse(_message.Message):
    __slots__ = ("output", "usage")
    OUTPUT_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    output: str
    usage: TokenUsage
    def __init__(self, output: _Optional[str] = ..., usage: _Optional[_Union[TokenUsage, _Mapping]] = ...) -> None: ...
