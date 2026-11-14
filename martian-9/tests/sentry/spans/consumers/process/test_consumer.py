from datetime import datetime
from threading import Event

import pytest
import rapidjson
from arroyo.backends.kafka import KafkaPayload
from arroyo.types import BrokerValue, Message, Partition, Topic

from sentry.spans.consumers.process.factory import ProcessSpansStrategyFactory


@pytest.mark.django_db(transaction=True)
def test_basic(monkeypatch):
    # Flush very aggressively to make test pass instantly
    monkeypatch.setattr("time.sleep", lambda _: None)

    topic = Topic("test")
    messages: list[KafkaPayload] = []
    message_received = Event()

    def append_message(message: KafkaPayload) -> None:
        messages.append(message)
        message_received.set()

    fac = ProcessSpansStrategyFactory(
        max_batch_size=10,
        max_batch_time=10,
        num_processes=1,
        input_block_size=None,
        output_block_size=None,
        produce_to_pipe=append_message,
    )

    commits = []

    def add_commit(offsets, force=False):
        commits.append(offsets)

    step = fac.create_with_partitions(add_commit, {Partition(topic, 0): 0})

    step.submit(
        Message(
            BrokerValue(
                partition=Partition(topic, 0),
                offset=1,
                payload=KafkaPayload(
                    None,
                    rapidjson.dumps(
                        {
                            "project_id": 12,
                            "span_id": "a" * 16,
                            "trace_id": "b" * 32,
                        }
                    ).encode("ascii"),
                    [],
                ),
                timestamp=datetime.now(),
            )
        )
    )

    step.poll()
    fac._flusher.current_drift.value = 9000  # "advance" our "clock"

    step.poll()
    assert message_received.wait(timeout=5), "Flusher did not produce the expected message"

    step.join()

    (msg,) = messages

    assert rapidjson.loads(msg.value) == {
        "spans": [
            {
                "data": {
                    "__sentry_internal_span_buffer_outcome": "different",
                },
                "is_segment": True,
                "project_id": 12,
                "segment_id": "aaaaaaaaaaaaaaaa",
                "span_id": "aaaaaaaaaaaaaaaa",
                "trace_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            },
        ],
    }


@pytest.mark.django_db(transaction=True)
def test_flusher_processes_limit(monkeypatch):
    """Test that flusher respects the max_processes limit"""
    # Flush very aggressively to make test pass instantly
    monkeypatch.setattr("time.sleep", lambda _: None)

    topic = Topic("test")
    messages: list[KafkaPayload] = []

    # Create factory with limited flusher processes
    fac = ProcessSpansStrategyFactory(
        max_batch_size=10,
        max_batch_time=10,
        num_processes=1,
        input_block_size=None,
        output_block_size=None,
        flusher_processes=2,  # Limit to 2 processes even if more shards
        produce_to_pipe=messages.append,
    )

    commits = []

    def add_commit(offsets, force=False):
        commits.append(offsets)

    # Create with 4 partitions/shards to test process sharing
    partitions = {Partition(topic, i): 0 for i in range(4)}
    step = fac.create_with_partitions(add_commit, partitions)

    # Verify that flusher uses at most 2 processes
    flusher = fac._flusher
    assert len(flusher.processes) == 2
    assert flusher.max_processes == 2
    assert flusher.num_processes == 2

    # Verify shards are distributed across processes
    total_shards = sum(len(shards) for shards in flusher.process_to_shards_map.values())
    assert total_shards == 4  # All 4 shards should be assigned

    step.join()
