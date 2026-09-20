class BencodeError(ValueError):
    def __init__(self, message, position=None):
        if position is not None:
            message = f"{message} at byte {position}"
        super().__init__(message)
        self.position = position

