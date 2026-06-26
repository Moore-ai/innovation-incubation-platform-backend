import sys
import os
from io import BytesIO
from fastapi import FastAPI, UploadFile, File, Form
from markitdown import MarkItDown
import uvicorn

app = FastAPI()
converter = MarkItDown()


@app.post("/convert")
async def convert(file: UploadFile = File(...), ext: str = Form("")):
    content = await file.read()
    try:
        result = converter.convert(BytesIO(content), extension=ext)
        return {"markdown": result.text_content}
    except Exception as e:
        return {"error": str(e)}


if __name__ == "__main__":
    port = int(sys.argv[1])
    config = uvicorn.Config(app, host="127.0.0.1", port=port, log_level="info")
    server = uvicorn.Server(config)
    server.run()
