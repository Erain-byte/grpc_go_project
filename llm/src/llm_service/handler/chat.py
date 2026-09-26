from collections.abc import AsyncIterator

import grpc

from llm.v1 import llm_pb2, llm_pb2_grpc
from llm_service.middleware.context import current_context


class ChatHandler(llm_pb2_grpc.LlmServiceServicer):
  

    async def Chat(self, request:llm_pb2.ChatRequest, context:grpc.aio.ServicerContext) :
            # TODO: Implement chat logic
        if not request.message.strip():
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "message is required")
        user_id = current_context().user_id
        
        if request.conversation_id and not request.conversation_id.strip():
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "conversation_id is required")
            return
        
        conversation_id = request.conversation_id.strip()

        reply =" user- {uid}-{conversation_id}:{request.message}"
        response = llm_pb2.ChatResponse(
            reply=reply,
            conversation_id=conversation_id,
            user_id=user_id,
            model=request.model or "mock-chat",
            usage=llm_pb2.TokenUsage(
                prompt_tokens=len(request.message),
                completion_tokens=len(reply),
                total_tokens=len(request.message) + len(reply),
            ),    
        )
        response.timestamp.GetCurrentTime()
        return response

    async def StreamChat(self,request: llm_pb2.StreamChatRequest, context: grpc.aio.ServicerContext) ->AsyncIterator[llm_pb2.StreamChatResponse]:
            if request.conversation_id and not request.conversation_id.strip():
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "conversation_id is required")
                return

            conversation_id = request.conversation_id.strip()
            #获取消息
            # 目前先按标准流程获取message后期对接框架后需要修改
            message = request.message
            if not  message:    
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "message is required")
                return
            #每次返回最多5个字符，不丢失原文中的所有内容
            for i in range(0,len(message),5):
                yield llm_pb2.StreamChatResponse(
                    delta=message[i:i+5],
                    conversation_id=conversation_id,
                    model=request.model or "mock-chat",
                    finished=False,
                )
            yield llm_pb2.StreamChatResponse(
                conversation_id=conversation_id,
                model=request.model or "mock-chat",
                finished=True,
                finish_reason="stop"
            )
