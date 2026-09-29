from llm_service.model.types import (
    ModelChunk,
    ModelRequest,
    ModelResponse,
)
class MockProvider:
    def __init__(self,model:str,delay:float=0.0,chunk_size:int=5):
           if delay <0.0:
              raise ValueError("delay must be positive")    
           if chunk_size <0:
              raise ValueError("chunk_size must be positive")
           self._model = model
           self._delay = delay
           self._chunk_size = chunk_size

    def chat(self, request: ModelRequest)->ModelRequest:
         ...